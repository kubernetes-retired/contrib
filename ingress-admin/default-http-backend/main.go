package main

import "net/http"

var out string = `
<html>
    <head>
        <meta http-equiv="content-type" content="text/html; charset=UTF-8">
        <title>page not found</title>
    </head>

    <body>
        <h1>Page Not Found</h1>
    </body>
</html>
`

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){
		w.WriteHeader(404)
		w.Write([]byte(out))
	})
	http.ListenAndServe(":80", nil)
	select{}
}