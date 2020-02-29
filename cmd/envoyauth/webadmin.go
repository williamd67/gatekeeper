package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

//WebAdminHTTPServer contains authorization data pointer
// so we can print its configuration later on
//
type WebAdminHTTPServer struct {
	a *authorizationServer
}

//StartWebAdminServer starts the admin web UI
//
func StartWebAdminServer(a authorizationServer) {
	w := WebAdminHTTPServer{}
	w.a = &a

	mux := http.NewServeMux()
	mux.HandleFunc("/", w.showIndexPage)
	mux.HandleFunc("/config_dump", w.configDump)
	mux.HandleFunc("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}).ServeHTTP)

	log.Printf("Webadmin listening on %s", w.a.config.WebAdminListen)
	go func() {
		log.Fatal(http.ListenAndServe(w.a.config.WebAdminListen, mux))
	}()
}

//configDump pretty prints the active configuration
//
func (wa *WebAdminHTTPServer) configDump(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// We must remove db password from configuration struct before showing
	configToPrint := wa.a.config
	configToPrint.DatabasePassword = ""

	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "\t")
	err := encoder.Encode(configToPrint)
	if err != nil {
		return
	}
	fmt.Fprint(w, buffer.String())
}

//showIndexPage produces the index page of the admin UI
//
func (wa *WebAdminHTTPServer) showIndexPage(w http.ResponseWriter, r *http.Request) {
	// The "/" pattern matches everything, we want to serve / only
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, adminIndexHTML)
}

//<link rel='shortcut icon' type='image/x-icon' href='data:image/x-icon;base64,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'/>

const adminIndexHTML = `
<head>
<title>APIAuth Admin</title>
<style>
.home-table {
  font-family: sans-serif;
  font-size: medium;
  border-collapse: collapse;
}

.home-row:nth-child(even) {
  background-color: #dddddd;
}

.home-data {
  border: 1px solid #dddddd;
  text-align: left;
  padding: 8px;
}

.home-form {
  margin-bottom: 0;
}
</style>

</head>
<body>
<h1>APIAuth Admin</h1>
<table class='home-table'>
<thead>
	<th class='home-data'>Command</th>
	<th class='home-data'>Description</th>
</thead>
<tbody>
	<tr class='home-row'>
		<td class='home-data'><a href='config_dump'>config dump</a></td>
		<td class='home-data'>print current active configuration</td>
	</tr>
	<tr class='home-row'>
		<td class='home-data'><a href='metrics'>metrics</a></td>
		<td class='home-data'>print auth server stats in prometheus format</td>
	</tr>
</tbody>
</table>
</body>
`
