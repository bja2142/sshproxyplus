mkdir -p static

go test -coverprofile cover.out
go tool cover -html=cover.out -o coverage.html

mv coverage.html static/coverage.html

godoc -url "http://localhost:6060/pkg/$(go list -m)" | egrep -v 'using module mode; GOMOD=' | sed 's+/lib/godoc/++g' | sed 's+href="/src/+href="http://+g' > static/index.html
godoc -url "http://localhost:6060/lib/godoc/style.css" > static/style.css
godoc -url "http://localhost:6060/lib/godoc/jquery.js" > static/jquery.js
godoc -url "http://localhost:6060/lib/godoc/godocs.js" > static/godocs.js
