mkdir -p public

go test -coverprofile cover.out 2>/dev/null 1>/dev/null
go tool cover -html=cover.out -o coverage.html

mv coverage.html public/coverage.html

godoc -url "http://localhost:6060/pkg/$(go list -m)" | egrep -v 'using module mode; GOMOD=' | sed 's+/lib/godoc/++g' | sed 's+href="/src/+href="http://+g' | sed 's+<body>+<body><h2><center><a href="coverage.html">Coverage Report</a></center></h2>+' > public/index.html
godoc -url "http://localhost:6060/lib/godoc/style.css" > public/style.css
godoc -url "http://localhost:6060/lib/godoc/jquery.js" > public/jquery.js
godoc -url "http://localhost:6060/lib/godoc/godocs.js" > public/godocs.js
