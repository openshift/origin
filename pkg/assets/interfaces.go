package assets

type AssetFunc func(path string) ([]byte, error)

type AssetDirFunc func(path string) ([]string, error)
