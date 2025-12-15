package utils

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

// CSVParser handles CSV file parsing
type CSVParser struct {
	filePath string
	headers  []string
	rows     []map[string]string
}

// NewCSVParser creates a new CSV parser
func NewCSVParser(filePath string) *CSVParser {
	return &CSVParser{
		filePath: filePath,
		rows:     make([]map[string]string, 0),
	}
}

// Parse reads and parses the CSV file
func (p *CSVParser) Parse() error {
	file, err := os.Open(p.filePath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV file: %w", err)
	}

	if len(records) == 0 {
		return fmt.Errorf("CSV file is empty")
	}

	// First row is headers
	p.headers = records[0]

	// Parse remaining rows
	for i := 1; i < len(records); i++ {
		if len(records[i]) != len(p.headers) {
			continue // Skip malformed rows
		}

		row := make(map[string]string)
		for j, header := range p.headers {
			row[header] = records[i][j]
		}
		p.rows = append(p.rows, row)
	}

	return nil
}

// GetRows returns all parsed rows
func (p *CSVParser) GetRows() []map[string]string {
	return p.rows
}

// GetHeaders returns the CSV headers
func (p *CSVParser) GetHeaders() []string {
	return p.headers
}

// GetValue returns a value from a row by column name
func GetValue(row map[string]string, columnName string) string {
	if val, exists := row[columnName]; exists {
		return val
	}
	return ""
}

// ApplyTransformation applies a transformation function to a field value
func ApplyTransformation(value string, funcName string, args interface{}) (interface{}, error) {
	switch funcName {
	case "toLower":
		return ToLower(value), nil

	case "toUpper":
		return ToUpper(value), nil

	case "removeNonAlphaNumeric":
		return RemoveNonAlphaNumeric(value), nil

	case "boolStringCompare":
		compareList, ok := args.([]interface{})
		if !ok {
			return false, fmt.Errorf("invalid args for boolStringCompare")
		}
		strList := make([]string, 0, len(compareList))
		for _, item := range compareList {
			if str, ok := item.(string); ok {
				strList = append(strList, str)
			}
		}
		return BoolStringCompare(value, strList), nil

	case "strStringCompare":
		// args should be a map with "arg" and "retVal" keys
		argsMap, ok := args.(map[string]interface{})
		if !ok {
			return value, fmt.Errorf("invalid args for strStringCompare")
		}

		argList, ok := argsMap["arg"].([]interface{})
		if !ok {
			return value, fmt.Errorf("invalid arg list for strStringCompare")
		}

		retValList, ok := argsMap["retVal"].([]interface{})
		if !ok {
			return value, fmt.Errorf("invalid retVal list for strStringCompare")
		}

		// Convert []interface{} to [][]string
		compareList := make([][]string, 0, len(argList))
		for _, item := range argList {
			subList, ok := item.([]interface{})
			if !ok {
				continue
			}
			strSubList := make([]string, 0, len(subList))
			for _, subItem := range subList {
				if str, ok := subItem.(string); ok {
					strSubList = append(strSubList, str)
				}
			}
			compareList = append(compareList, strSubList)
		}

		// Convert []interface{} to []string
		retVals := make([]string, 0, len(retValList))
		for _, item := range retValList {
			if str, ok := item.(string); ok {
				retVals = append(retVals, str)
			}
		}

		return StrStringCompare(value, compareList, retVals), nil

	case "splitString":
		delimiter := ","
		if args != nil {
			if delim, ok := args.(string); ok {
				delimiter = delim
			}
		}
		return SplitString(value, delimiter), nil

	case "parseInt":
		val, err := strconv.Atoi(value)
		if err != nil {
			return 0, err
		}
		return val, nil

	case "parseBool":
		return BoolStringCompare(value, []string{"true", "yes", "1", "on"}), nil

	default:
		return value, fmt.Errorf("unknown transformation function: %s", funcName)
	}
}
