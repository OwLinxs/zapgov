package flow

import "testing"

func TestRender(t *testing.T) {
	got := render("Olá {{nome}}, sua consulta é {{data}}", map[string]string{"nome": "João", "data": "05/06"})
	want := "Olá João, sua consulta é 05/06"
	if got != want {
		t.Errorf("render = %q, quer %q", got, want)
	}
}

func TestEvalCond(t *testing.T) {
	cases := []struct {
		got, op, want string
		exp           bool
	}{
		{"João", "eq", "joão", true}, // case-insensitive
		{"João", "eq", "Maria", false},
		{" sim ", "eq", "sim", true}, // trim
		{"preciso de ajuda", "contains", "AJUDA", true},
		{"oi", "contains", "tchau", false},
	}
	for _, c := range cases {
		if got := evalCond(c.got, c.op, c.want); got != c.exp {
			t.Errorf("evalCond(%q,%q,%q) = %v, quer %v", c.got, c.op, c.want, got, c.exp)
		}
	}
}

func TestGraphNavigation(t *testing.T) {
	raw := []byte(`{
	  "nodes":[
	    {"id":"start","type":"start","data":{}},
	    {"id":"menu","type":"menu","data":{}},
	    {"id":"a","type":"message","data":{}}
	  ],
	  "edges":[
	    {"source":"start","target":"menu"},
	    {"source":"menu","sourceHandle":"1","target":"a"}
	  ]
	}`)
	g, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse erro: %v", err)
	}
	if s := g.start(); s == nil || s.ID != "start" {
		t.Fatal("não achou nó start")
	}
	if g.next("start") != "menu" {
		t.Errorf("next(start) = %q, quer menu", g.next("start"))
	}
	if g.target("menu", "1") != "a" {
		t.Errorf("target(menu,1) = %q, quer a", g.target("menu", "1"))
	}
	if g.target("menu", "9") != "" {
		t.Error("opção inexistente devia devolver vazio")
	}
}
