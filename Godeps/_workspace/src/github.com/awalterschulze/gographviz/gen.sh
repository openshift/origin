gocc -o ../ -p . dot.bnf
cp ./parser/parser.temp ./parser/parser.go
gofix parser/tables.go
gofix token/token.go
