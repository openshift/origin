package db

import (
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Sqlit is a Sqlit helper for executing commands.
type Sqlit struct {
	driverName string
}

type OperatorBundle struct {
	name       string
	bundlepath string
	version    string
}

type Channel struct {
	entry_id            int64
	channel_name        string
	package_name        string
	operatorbundle_name string
	replaces            string
	depth               int
}

type Package struct {
	name            string
	default_channel string
}

type Image struct {
	image               string
	operatorbundle_name string
}

// NewSqlit creates a new sqlit instance.
func NewSqlit() *Sqlit {
	return &Sqlit{
		driverName: "sqlite3",
	}
}

// QueryDB executes an sqlit query as an ordinary user and returns the result.
func (c *Sqlit) QueryDB(dbFilePath string, query string) (*sql.Rows, error) {
	if _, err := os.Stat(dbFilePath); os.IsNotExist(err) {
		e2e.Logf("file %s do not exist", dbFilePath)
		return nil, err
	}
	database, err := sql.Open(c.driverName, dbFilePath)
	if err != nil {
		return nil, err
	}
	defer database.Close()
	rows, err := database.Query(query)
	if err != nil {
		return nil, err
	}
	return rows, err
}

// QueryOperatorBundle executes an sqlit query as an ordinary user and returns the result.
func (c *Sqlit) QueryOperatorBundle(dbFilePath string) ([]OperatorBundle, error) {
	rows, err := c.QueryDB(dbFilePath, "SELECT name,bundlepath,version FROM operatorbundle")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var OperatorBundles []OperatorBundle
	var name string
	var bundlepath string
	var version string
	for rows.Next() {
		rows.Scan(&name, &bundlepath, &version)
		OperatorBundles = append(OperatorBundles, OperatorBundle{name: name, bundlepath: bundlepath, version: version})
		e2e.Logf("OperatorBundles: name: %s,bundlepath: %s, version: %s", name, bundlepath, version)
	}
	return OperatorBundles, nil
}

// CheckOperatorBundlePathExist is to check the OperatorBundlePath exist
func (c *Sqlit) CheckOperatorBundlePathExist(dbFilePath string, bundlepath string) (bool, error) {
	OperatorBundles, err := c.QueryOperatorBundle(dbFilePath)
	if err != nil {
		return false, err
	}
	for _, OperatorBundle := range OperatorBundles {
		if strings.Compare(OperatorBundle.bundlepath, bundlepath) == 0 {
			return true, nil
		}
	}
	return false, nil
}

// CheckOperatorBundleNameExist is to check the OperatorBundleName exist
func (c *Sqlit) CheckOperatorBundleNameExist(dbFilePath string, bundleName string) (bool, error) {
	OperatorBundles, err := c.QueryOperatorBundle(dbFilePath)
	if err != nil {
		return false, err
	}
	for _, OperatorBundle := range OperatorBundles {
		if strings.Compare(OperatorBundle.name, bundleName) == 0 {
			return true, nil
		}
	}
	return false, nil
}

// QueryOperatorChannel executes an sqlit query as an ordinary user and returns the result.
func (c *Sqlit) QueryOperatorChannel(dbFilePath string) ([]Channel, error) {
	rows, err := c.QueryDB(dbFilePath, "select * from channel_entry;")
	var (
		Channels            []Channel
		entry_id            int64
		channel_name        string
		package_name        string
		operatorbundle_name string
		replaces            string
		depth               int
	)
	defer rows.Close()
	if err != nil {
		return Channels, err
	}

	for rows.Next() {
		rows.Scan(&entry_id, &channel_name, &package_name, &operatorbundle_name, &replaces, &depth)
		Channels = append(Channels, Channel{entry_id: entry_id,
			channel_name:        channel_name,
			package_name:        package_name,
			operatorbundle_name: operatorbundle_name,
			replaces:            replaces,
			depth:               depth})
	}
	return Channels, nil
}

// QueryPackge executes an sqlit query as an ordinary user and returns the result.
func (c *Sqlit) QueryPackge(dbFilePath string) ([]Package, error) {
	rows, err := c.QueryDB(dbFilePath, "select * from package;")
	var (
		Packages        []Package
		name            string
		default_channel string
	)
	defer rows.Close()
	if err != nil {
		return Packages, err
	}

	for rows.Next() {
		rows.Scan(&name, &default_channel)
		Packages = append(Packages, Package{name: name,
			default_channel: default_channel})
	}
	return Packages, nil
}

// QueryRelatedImage executes an sqlit query as an ordinary user and returns the result.
func (c *Sqlit) QueryRelatedImage(dbFilePath string) ([]Image, error) {
	rows, err := c.QueryDB(dbFilePath, "select * from related_image;")
	var (
		relatedImages       []Image
		image               string
		operatorbundle_name string
	)
	defer rows.Close()
	if err != nil {
		return relatedImages, err
	}

	for rows.Next() {
		rows.Scan(&image, &operatorbundle_name)
		relatedImages = append(relatedImages, Image{image: image,
			operatorbundle_name: operatorbundle_name})
	}
	return relatedImages, nil
}

// GetOperatorChannelByColumn
func (c *Sqlit) GetOperatorChannelByColumn(dbFilePath string, column string) ([]string, error) {
	channels, err := c.QueryOperatorChannel(dbFilePath)
	if err != nil {
		return nil, err
	}
	var channelList []string
	for _, channel := range channels {
		//valueDB := reflections.GetField(channel, column)
		value := reflect.Indirect(reflect.ValueOf(&channel)).FieldByName(column)
		channelList = append(channelList, value.String())
	}
	return channelList, nil
}

func (c *Sqlit) Query(dbFilePath string, table string, column string) ([]string, error) {
	var valueList []string
	switch table {
	case "operatorbundle":
		result, err := c.QueryOperatorBundle(dbFilePath)
		if err != nil {
			return nil, err
		}
		for _, channel := range result {
			value := reflect.Indirect(reflect.ValueOf(&channel)).FieldByName(column)
			valueList = append(valueList, value.String())
		}
		return valueList, nil
	case "channel_entry":
		result, err := c.QueryOperatorChannel(dbFilePath)
		if err != nil {
			return nil, err
		}
		for _, channel := range result {
			value := reflect.Indirect(reflect.ValueOf(&channel)).FieldByName(column)
			valueList = append(valueList, value.String())
		}
		return valueList, nil
	case "package":
		result, err := c.QueryPackge(dbFilePath)
		if err != nil {
			return nil, err
		}
		for _, packageIndex := range result {
			value := reflect.Indirect(reflect.ValueOf(&packageIndex)).FieldByName(column)
			valueList = append(valueList, value.String())
		}
		return valueList, nil
	case "related_image":
		result, err := c.QueryRelatedImage(dbFilePath)
		if err != nil {
			return nil, err
		}
		for _, imageIndex := range result {
			value := reflect.Indirect(reflect.ValueOf(&imageIndex)).FieldByName(column)
			valueList = append(valueList, value.String())
		}
		return valueList, nil
	default:
		err := fmt.Errorf("do not support to query table %s", table)
		return nil, err
	}
}

// DBHas
func (c *Sqlit) DBHas(dbFilePath string, table string, column string, valueList []string) (bool, error) {
	valueListDB, err := c.Query(dbFilePath, table, column)
	if err != nil {
		return false, err
	}
	return contains(valueListDB, valueList), nil
}

// DBMatch
func (c *Sqlit) DBMatch(dbFilePath string, table string, column string, valueList []string) (bool, error) {
	valueListDB, err := c.Query(dbFilePath, table, column)
	if err != nil {
		return false, err
	}
	return match(valueListDB, valueList), nil
}

func contains(stringList1 []string, stringList2 []string) bool {
	for _, stringIndex2 := range stringList2 {
		containFlag := false
		for _, stringIndex1 := range stringList1 {
			if strings.Compare(stringIndex1, stringIndex2) == 0 {
				containFlag = true
				break
			}
		}
		if !containFlag {
			e2e.Logf("[%s] do not contain [%s]", strings.Join(stringList1, ","), strings.Join(stringList2, ","))
			return false
		}
	}
	return true
}

func match(stringList1 []string, stringList2 []string) bool {
	if len(stringList1) != len(stringList2) {
		return false
	}
	for _, stringIndex2 := range stringList2 {
		containFlag := false
		for _, stringIndex1 := range stringList1 {
			if strings.Compare(stringIndex1, stringIndex2) == 0 {
				containFlag = true
				break
			}
		}
		if !containFlag {
			e2e.Logf("[%s] do not equal to [%s]", strings.Join(stringList1, ","), strings.Join(stringList2, ","))
			return false
		}
	}
	return true
}
