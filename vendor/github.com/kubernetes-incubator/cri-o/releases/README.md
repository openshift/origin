## `releases` ##

```
make install-tools
git tag vX.Y.Z <commit>
touch releases/vX.Y.Z.toml
make release-note release=releases/vX.Y.Z.toml
```
