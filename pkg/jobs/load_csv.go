package jobs

import (
	"context"
	"fmt"
	"strings"

	"ElasticObservability/pkg/logger"
	"ElasticObservability/pkg/types"
	"ElasticObservability/pkg/utils"
)

// LoadFromMasterCSV loads cluster data from CSV file
func LoadFromMasterCSV(ctx context.Context, params map[string]interface{}) error {
	logger.JobInfo("loadFromMasterCSV", "Starting CSV load job")

	// Get CSV file name from parameters
	csvFileName, ok := params["csv_fileName"].(string)
	if !ok || csvFileName == "" {
		return fmt.Errorf("csv_fileName parameter is required")
	}

	// Get input mapping
	inputMapping, ok := params["inputMapping"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("inputMapping parameter is required")
	}

	// Parse CSV file
	parser := utils.NewCSVParser(csvFileName)
	if err := parser.Parse(); err != nil {
		return fmt.Errorf("failed to parse CSV: %w", err)
	}

	rows := parser.GetRows()
	logger.JobInfo("loadFromMasterCSV", "Parsed %d rows from CSV", len(rows))

	// Process each row
	addedClusters := 0
	addedNodes := 0
	skippedRows := 0

	for rowIdx, row := range rows {
		// Get cluster name
		clusterName := getClusterNameFromRow(row, inputMapping)
		if clusterName == "" {
			logger.JobWarn("loadFromMasterCSV", "Row %d: Empty cluster name, skipping", rowIdx+1)
			skippedRows++
			continue
		}

		// Get or create cluster
		types.ClustersMu.Lock()
		cluster, exists := types.AllClusters[clusterName]
		if !exists {
			cluster = &types.ClusterData{
				ClusterName: clusterName,
				Active:      true, // Default to active
				Nodes:       make([]*types.Node, 0),
			}
			types.AllClusters[clusterName] = cluster
			types.AllClustersList = append(types.AllClustersList, clusterName)
			addedClusters++
			logger.JobInfo("loadFromMasterCSV", "Created new cluster: %s", clusterName)
		}
		types.ClustersMu.Unlock()

		// Process constant values
		if err := applyConstantValues(cluster, inputMapping); err != nil {
			logger.JobWarn("loadFromMasterCSV", "Row %d: Failed to apply constants: %v", rowIdx+1, err)
		}

		// Process straight mappings (cluster level)
		if err := applyStraightMappingsCluster(cluster, row, inputMapping); err != nil {
			logger.JobWarn("loadFromMasterCSV", "Row %d: Failed to apply straight mappings: %v", rowIdx+1, err)
		}

		// Process derived fields (cluster level)
		if err := applyDerivedFieldsCluster(cluster, row, inputMapping); err != nil {
			logger.JobWarn("loadFromMasterCSV", "Row %d: Failed to apply derived fields: %v", rowIdx+1, err)
		}

		// Create and add node
		node := &types.Node{
			Port:       "9200", // default
			KibanaPort: "5601", // default
		}

		// Process node fields
		if err := applyStraightMappingsNode(node, row, inputMapping); err != nil {
			logger.JobWarn("loadFromMasterCSV", "Row %d: Failed to apply node mappings: %v", rowIdx+1, err)
		}

		if err := applyDerivedFieldsNode(node, row, inputMapping); err != nil {
			logger.JobWarn("loadFromMasterCSV", "Row %d: Failed to apply node derived fields: %v", rowIdx+1, err)
		}

		// Check if node already exists
		nodeExists := false
		for _, existingNode := range cluster.Nodes {
			if existingNode.HostName == node.HostName {
				nodeExists = true
				logger.JobInfo("loadFromMasterCSV", "Row %d: Node %s already exists in cluster %s, skipping",
					rowIdx+1, node.HostName, clusterName)
				skippedRows++
				break
			}
		}

		if !nodeExists && node.HostName != "" {
			cluster.Nodes = append(cluster.Nodes, node)
			addedNodes++
		}
	}

	logger.JobInfo("loadFromMasterCSV", "Completed: Added %d clusters, %d nodes. Skipped %d rows",
		addedClusters, addedNodes, skippedRows)

	return nil
}

func getClusterNameFromRow(row map[string]string, inputMapping map[string]interface{}) string {
	straight, ok := inputMapping["straight"].(map[string]interface{})
	if !ok {
		return ""
	}

	clusterNameCol, ok := straight["clusterName"].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(utils.GetValue(row, clusterNameCol))
}

func applyConstantValues(cluster *types.ClusterData, inputMapping map[string]interface{}) error {
	constants, ok := inputMapping["constant"].(map[string]interface{})
	if !ok {
		return nil // No constants defined
	}

	for field, value := range constants {
		switch field {
		case "insecureTLS":
			if val, ok := value.(bool); ok {
				cluster.InsecureTLS = val
			}
		case "port":
			// Port is handled at node level
		}
	}

	return nil
}

func applyStraightMappingsCluster(cluster *types.ClusterData, row map[string]string, inputMapping map[string]interface{}) error {
	straight, ok := inputMapping["straight"].(map[string]interface{})
	if !ok {
		return nil
	}

	for field, column := range straight {
		columnStr, ok := column.(string)
		if !ok {
			continue
		}

		value := utils.GetValue(row, columnStr)
		if value == "" {
			continue
		}

		switch field {
		case "clusterName":
			// Already handled
		case "clusterSAN":
			cluster.ClusterSAN = utils.SplitString(value, ",")
		case "kibanaSAN":
			cluster.KibanaSAN = utils.SplitString(value, ",")
		case "owner":
			cluster.Owner = value
		case "clusterUUID":
			cluster.ClusterUUID = value
		case "currentEndpoint":
			cluster.CurrentEndpoint = value
		case "zoneIdentifier":
			cluster.ZoneIdentifier = value
		}
	}

	return nil
}

func applyDerivedFieldsCluster(cluster *types.ClusterData, row map[string]string, inputMapping map[string]interface{}) error {
	derived, ok := inputMapping["derived"].([]interface{})
	if !ok {
		return nil
	}

	for _, item := range derived {
		derivedField, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		field, _ := derivedField["field"].(string)
		column, _ := derivedField["column"].(string)
		function, _ := derivedField["function"].(string)
		arg := derivedField["arg"]
		retVal, _ := derivedField["retVal"].([]interface{})

		value := utils.GetValue(row, column)
		if value == "" {
			continue
		}

		var result interface{}
		var err error

		if function == "strStringCompare" && retVal != nil {
			// Special handling for strStringCompare
			argsMap := map[string]interface{}{
				"arg":    arg,
				"retVal": retVal,
			}
			result, err = utils.ApplyTransformation(value, function, argsMap)
		} else {
			result, err = utils.ApplyTransformation(value, function, arg)
		}

		if err != nil {
			logger.JobWarn("loadFromMasterCSV", "Failed to apply transformation %s: %v", function, err)
			continue
		}

		switch field {
		case "active":
			if val, ok := result.(bool); ok {
				cluster.Active = val
			}
		case "env":
			if val, ok := result.(string); ok {
				cluster.Env = val
			}
		}
	}

	return nil
}

func applyStraightMappingsNode(node *types.Node, row map[string]string, inputMapping map[string]interface{}) error {
	straight, ok := inputMapping["straight"].(map[string]interface{})
	if !ok {
		return nil
	}

	for field, column := range straight {
		columnStr, ok := column.(string)
		if !ok {
			continue
		}

		value := utils.GetValue(row, columnStr)
		if value == "" {
			continue
		}

		switch field {
		case "hostName":
			node.HostName = value
		case "port":
			node.Port = value
		case "kibanaPort":
			node.KibanaPort = value
		case "logstashPort":
			node.LogstashPort = value
		case "ipAddress":
			node.IPAddress = value
		case "zone":
			node.Zone = value
		case "dataCenter":
			node.DataCenter = value
		case "rack":
			node.Rack = value
		case "nodeTier":
			node.NodeTier = value
		}
	}

	return nil
}

func applyDerivedFieldsNode(node *types.Node, row map[string]string, inputMapping map[string]interface{}) error {
	derived, ok := inputMapping["derived"].([]interface{})
	if !ok {
		return nil
	}

	for _, item := range derived {
		derivedField, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		field, _ := derivedField["field"].(string)
		column, _ := derivedField["column"].(string)
		function, _ := derivedField["function"].(string)
		arg := derivedField["arg"]

		value := utils.GetValue(row, column)
		if value == "" {
			continue
		}

		result, err := utils.ApplyTransformation(value, function, arg)
		if err != nil {
			logger.JobWarn("loadFromMasterCSV", "Failed to apply transformation %s: %v", function, err)
			continue
		}

		switch field {
		case "type":
			if val, ok := result.([]string); ok {
				node.Type = val
			} else if strVal, ok := result.(string); ok {
				node.Type = utils.GetNodeTypes(strVal)
			}
		}
	}

	return nil
}
