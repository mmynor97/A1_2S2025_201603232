package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	prolog "github.com/ichiban/prolog"
)

// ===== Tipos =====

type SintomaInput struct {
	Nombre    string `json:"nombre"`
	Severidad string `json:"severidad"`
}

type AnalyzeReq struct {
	Sintomas []SintomaInput `json:"sintomas"`
	Alergias []string       `json:"alergias"`
	Cronicos []string       `json:"cronicos"`
}

type AnalyzeResp struct {
	Resultados []map[string]interface{} `json:"resultados"`
}

// ===== Utils =====

func mustRead(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("no se pudo leer %s: %v", path, err)
	}
	return string(b)
}

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r)
	}
}

func toPLTupleList(pares []SintomaInput) string {
	if len(pares) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range pares {
		if i > 0 {
			b.WriteByte(',')
		}
		name := atomize(s.Nombre)
		sev := atomize(s.Severidad)
		b.WriteByte('(')
		b.WriteString(name)
		b.WriteByte(',')
		b.WriteString(sev)
		b.WriteByte(')')
	}
	b.WriteByte(']')
	return b.String()
}

func toPLAtomList(xs []string) string {
	if len(xs) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range xs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(atomize(x))
	}
	b.WriteByte(']')
	return b.String()
}

func atomize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r)
		case r >= '0' && r <= '9':
			out = append(out, r)
		case r == '_' || r == ' ' || r == '-':
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "desconocido"
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = append([]rune{'x', '_'}, out...)
	}
	return string(out)
}

// ===== main =====

func main() {
	vm := prolog.New(nil, nil)

	plPath := filepath.Join("prolog", "medi_logic.pl")
	code := mustRead(plPath)
	if err := vm.Exec(code, nil); err != nil {
		log.Fatalf("Error cargando medi_logic.pl: %v", err)
	}

	http.HandleFunc("/health", withCORS(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	http.HandleFunc("/admin/export", withCORS(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, plPath)
	}))

	http.HandleFunc("/analyze", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "solo POST", http.StatusMethodNotAllowed)
			return
		}

		var req AnalyzeReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		sv := toPLTupleList(req.Sintomas)
		al := toPLAtomList(req.Alergias)
		cr := toPLAtomList(req.Cronicos)

		q := fmt.Sprintf(`consulta_item(%s,%s,%s, Enf, Afin, Med, Urg).`, sv, al, cr)

		solutions, err := vm.Query(q)
		if err != nil {
			http.Error(w, fmt.Sprintf("error al consultar: %v", err), http.StatusInternalServerError)
			return
		}
		defer solutions.Close()

		var out []map[string]interface{}
		for solutions.Next() {

			var row struct {
				Enf  string
				Afin int64
				Med  string
				Urg  string
			}

			if err := solutions.Scan(&row); err != nil {
				http.Error(w, fmt.Sprintf("error al leer solución: %v", err), http.StatusInternalServerError)
				return
			}

			out = append(out, map[string]interface{}{
				"enfermedad":  row.Enf,
				"afinidad":    row.Afin,
				"medicamento": row.Med,
				"urgencia":    row.Urg,
			})
		}
		if err := solutions.Err(); err != nil {
			http.Error(w, fmt.Sprintf("error en soluciones: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AnalyzeResp{Resultados: out})
	}))

	log.Println("MediLogic backend en http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
