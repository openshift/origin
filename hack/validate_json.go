package main

import (
    "errors"
    "fmt"
    "github.com/xeipuuv/gojsonschema"
    "path/filepath"
    "os"
)

func ValidateFile(path string, info os.FileInfo, err error) error{
    schemaDocument, err := gojsonschema.NewJsonSchemaDocument("https://github.com/json-schema/json-schema/blob/master/draft-04/hyper-schema.json")
    if err != nil {
        return err
    }

    // Loads the JSON to validate from a local file
    jsonDocument, err := gojsonschema.GetFileJson(path)
    if err != nil {
        return err
    }

    result := schemaDocument.Validate(jsonDocument)

    if result.Valid() {
        return nil
    } else {
        return errors.New("Invalid JSON file.")
    }
}

func main() {
    // Validate each file from the directory
    directory := os.Args[1]
    err := filepath.Walk(directory, ValidateFile)

    if err != nil {
        fmt.Println("JSON files are valid.")
    } else {
        fmt.Println("Invalid JSON files. Errors: %v", err)
    }
}
