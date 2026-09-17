package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	MO "github.com/igroene/mongodboperatorcalculator/src/mongodboperatorcalculator"
)

func main() {
	addr := "0.0.0.0:" + strconv.Itoa(8080)
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}
	http.HandleFunc("/supported", supported)
	http.HandleFunc("/calculator", calculator)
	fmt.Println(http.ListenAndServe(addr, nil))
}
func supported(w http.ResponseWriter, r *http.Request) {
	var c MO.Configuration
	c.Init()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}
func calculator(w http.ResponseWriter, r *http.Request) {
	body, e := io.ReadAll(r.Body)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	var req MO.ConfigurationRequest
	if e = json.Unmarshal(body, &req); e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	var c MO.Configuration
	c.Init()
	var calc MO.MongoDBOperatorCalculator
	req = calc.Init(req, c)
	e, msg, f := calc.GetCalculate()
	if req.Output == MO.ResultOutputFormatHuman {
		out, _ := calc.GetHumanOutput(msg, req, f)
		w.Header().Set("Content-Type", "text/plain")
		w.Write(out.Bytes())
	} else {
		out, _ := calc.GetJSONOutput(msg, req, f)
		w.Header().Set("Content-Type", "application/json")
		w.Write(out.Bytes())
	}
	_ = e
}
