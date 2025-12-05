package dataloader

import (
	"encoding/json"
	"fmt"
	"os"
)

type DataType = string

const (
	DataTypeFloat64 DataType = "float64"
	DataTypeString  DataType = "string"
	DataTypeInteger DataType = "int64"
	// RFC3339  based value "2006-01-02T15:04:05Z07:02
	DataTypeTimestamp DataType = "timestamp"
	DataTypeJSON      DataType = "json"

	// files that end with this suffix will be automatically written to the specified table name via ci-data-loader
	AutoDataLoaderSuffix = "autodl.json"
)

type DataFile struct {
	// Table name to be created / updated with the corresponding data
	TableName string `json:"table_name"`
	// Schema identifying the data types associated with the row values
	// JobRunName, PartitionTime and Source will be provided by default and do not need to be specified here
	// Schema defined here are optional columns, unless used as PartitionColumn
	// New columns will be added but columns that get removed here will *not* be deleted
	// from the table in order to preserve integrity across releases
	// However as optional columns the data does not have to be
	// included if no longer necessary
	// if breaking changes are needed best to define a new table name
	Schema map[string]DataType `json:"schema"`
	// If the existing row key differs from the specified schema column name you need to map a row key to a different schema name rowKey->newName
	SchemaMapping map[string]string `json:"schema_mapping"`
	// The data to be uploaded
	Rows []map[string]string `json:"rows"`

	// Optional
	// Depending on the size of your data the rows might have to be chunked
	// when writing.  Default chunk size is 5k rows.
	// If the row data is large this can be changed to make smaller chunks
	ChunkSize int `json:"chunk_size"`

	// ExpirationDays and PartitionColumn will only
	// be used when first creating the table
	// if the table exists changing these
	// values will not update the table
	// Default expiration days is 365
	ExpirationDays int `json:"expiration_days"`
	// A partition column, PartitionTime, will automatically be added with the value
	// of the file creation timestamp.  If your data has a timestamp value already
	// it can be specified as the partition column instead
	// and the default PartitionTime will be omitted
	PartitionColumn string `json:"partition_column"`
}

func WriteDataFile(filename string, dataFile DataFile) error {
	jsonContent, err := json.MarshalIndent(dataFile, "", "    ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filename, jsonContent, 0644); err != nil {
		return fmt.Errorf("failed to write %v: %w", filename, err)
	}
	return nil
}
