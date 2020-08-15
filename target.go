package main

import (
	"encoding/json"
	"hash"
)

type Path string

func (p Path) String() string { return string(p) }

type GlobGroup []string

func (gg GlobGroup) String() string {
	return jsonSprint(struct {
		Globs []string `json:"globs"`
	}{[]string(gg)})
}

type String string

func (s String) String() string { return string(s) }

type Substitution struct {
	Key   string
	Value Arg
}

type Sub struct {
	Format        string
	Substitutions []Substitution
}

func (s *Sub) String() string {
	data, _ := json.MarshalIndent(s, "", "    ")
	return string(data)
}

type Arg interface {
	String() string
	Hash32(hash.Hash32)
	freezeArg(*freezer) (ArgValue, error)
}

type ArgValue struct {
	Value       string
	Hash        []byte
	Derivations []*Derivation
}

type Target struct {
	Name    string
	Builder string
	Args    []Arg
	Env     []string
}

func (t *Target) String() string { return jsonSprint(t) }

func jsonSprint(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		panic(err)
	}
	return string(data)
}
