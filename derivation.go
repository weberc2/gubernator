package main

import "encoding/json"

type Derivation struct {
	ID           string
	Hash         []byte
	Dependencies []*Derivation
	Builder      string
	Args         []string
	Env          []string
}

func (d *Derivation) String() string {
	data, _ := json.MarshalIndent(
		struct {
			Type  string
			Value interface{}
		}{Type: "Derivation", Value: d},
		"",
		"    ",
	)
	return string(data)
}
