package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	prolog "github.com/ichiban/prolog"
)

//
// ======== Tipos de datos (API) ========
//

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

//
// ======== Modelo de conocimiento (Admin/CRUD) ========
//

type Symptom struct {
	Name string `json:"name"`
}

type Caract struct {
	Symptom string `json:"symptom"` // nombre del síntoma
	Peso    int    `json:"peso"`    // 1..3
}

type Disease struct {
	Name            string   `json:"name"`
	Tipo            string   `json:"tipo"`    // viral, cronico, etc.
	Sistema         string   `json:"sistema"` // respiratorio, etc.
	Descripcion     string   `json:"descripcion"`
	Caracteristicas []Caract `json:"caracteristicas"`
}

type Medication struct {
	Name   string   `json:"name"`
	Treats []string `json:"treats"` // enfermedades
}

type ContraAlergia struct {
	Med     string `json:"med"`
	Alergia string `json:"alergia"`
}

type ContraCronico struct {
	Med     string `json:"med"`
	Cronico string `json:"cronico"`
}

type Knowledge struct {
	Symptoms       []Symptom       `json:"symptoms"`
	Diseases       []Disease       `json:"diseases"`
	Meds           []Medication    `json:"meds"`
	ContraAlergias []ContraAlergia `json:"contraAlergias"`
	ContraCronicos []ContraCronico `json:"contraCronicos"`
}

//
// ======== Estado global ========
//

var (
	adminToken = getenv("ADMIN_TOKEN", "admin123")
	plPath     = filepath.Join("prolog", "medi_logic.pl")

	vm   *prolog.Interpreter
	kb   Knowledge
	mu   sync.Mutex
	logp = log.Printf
)

//
// ======== Main ========
//

func main() {
	ensureDirs()

	// KB por defecto -> generar .pl -> cargar VM
	kb = defaultKB()
	code := buildPL(kb)
	if err := os.WriteFile(plPath, []byte(code), 0644); err != nil {
		log.Fatalf("No se pudo escribir %s: %v", plPath, err)
	}
	if err := reloadVM(code); err != nil {
		log.Fatalf("Error cargando Prolog: %v", err)
	}

	// Rutas
	http.HandleFunc("/health", withCORS(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "ok")
	}))

	http.HandleFunc("/analyze", withCORS(handleAnalyze))

	// Admin
	http.HandleFunc("/admin/export", withCORS(auth(handleExportPL)))
	http.HandleFunc("/admin/kb", withCORS(auth(handleKB)))              // GET/POST
	http.HandleFunc("/admin/upload-pl", withCORS(auth(handleUploadPL))) // POST multipart/simple
	http.HandleFunc("/admin/rpa/ingest", withCORS(auth(handleRPAIngest)))

	log.Println("MediLogic backend en http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

//
// ======== Handlers ========
//

func handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "solo POST", http.StatusMethodNotAllowed)
		return
	}
	var req AnalyzeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Construir términos Prolog [(s,sev),...], [a1,a2], [c1,c2]
	sv := toPLTupleList(req.Sintomas)
	als := toPLAtomList(req.Alergias)
	crs := toPLAtomList(req.Cronicos)

	q := fmt.Sprintf(`consulta_item(%s,%s,%s, Enf, Afin, Med, Urg).`, sv, als, crs)

	mu.Lock()
	solutions, err := vm.Query(q)
	mu.Unlock()
	if err != nil {
		http.Error(w, fmt.Sprintf("error al consultar: %v", err), http.StatusInternalServerError)
		return
	}
	defer solutions.Close()

	var out []map[string]interface{}

	for solutions.Next() {
		var row struct {
			Enf  string
			Afin int64 // si prefieres afinidad real, cambia a float64 en PL y aquí
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
	_ = json.NewEncoder(w).Encode(AnalyzeResp{Resultados: out})
}

func handleExportPL(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, plPath)
}

func handleKB(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(kb)

	case http.MethodPost:
		var in Knowledge
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		code := buildPL(in)
		if err := os.WriteFile(plPath, []byte(code), 0644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := reloadVM(code); err != nil {
			http.Error(w, "no se pudo recargar Prolog", http.StatusInternalServerError)
			return
		}
		mu.Lock()
		kb = in
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
	}
}

func handleUploadPL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "solo POST", http.StatusMethodNotAllowed)
		return
	}
	// Soporta multipart/form-data y text/plain
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "multipart inválido", http.StatusBadRequest)
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "archivo faltante 'file'", http.StatusBadRequest)
			return
		}
		defer f.Close()
		b, _ := io.ReadAll(f)
		if err := os.WriteFile(plPath, b, 0644); err != nil {
			http.Error(w, "no se pudo escribir .pl", http.StatusInternalServerError)
			return
		}
		if err := reloadVM(string(b)); err != nil {
			http.Error(w, "no se pudo recargar Prolog", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// Plano
	body, _ := io.ReadAll(r.Body)
	if len(body) == 0 {
		http.Error(w, "body vacío", http.StatusBadRequest)
		return
	}
	if err := os.WriteFile(plPath, body, 0644); err != nil {
		http.Error(w, "no se pudo escribir .pl", http.StatusInternalServerError)
		return
	}
	if err := reloadVM(string(body)); err != nil {
		http.Error(w, "no se pudo recargar Prolog", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleRPAIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "solo POST (text/plain)", http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(r.Body)
	txt := string(body)

	parsed := parseRPAFile(txt)

	// Actualiza KB y recarga
	mu.Lock()
	applyParsedToKB(&kb, parsed)
	code := buildPL(kb)
	_ = os.WriteFile(plPath, []byte(code), 0644)
	mu.Unlock()

	if err := reloadVM(code); err != nil {
		http.Error(w, "no se pudo recargar Prolog", http.StatusInternalServerError)
		return
	}

	report := buildRPAReport(parsed)
	if err := deliverReport(report); err != nil {
		logp("SMTP no disponible, guardando informe local. Error: %v", err)
		saveReportToDisk(report)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(report))
}

//
// ======== Middleware / util ========
//

func auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if tk := r.Header.Get("X-Admin-Token"); tk != "" && tk == adminToken {
			h(w, r)
			return
		}
		if r.URL.Query().Get("token") == adminToken {
			h(w, r)
			return
		}
		http.Error(w, "no autorizado", http.StatusUnauthorized)
	}
}

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r)
	}
}

func reloadVM(code string) error {
	vm = prolog.New(nil, nil)
	// Importante: Exec NO lleva segundo argumento
	return vm.Exec(code)
}

//
// ======== Helpers de construcción de términos ========
//

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
	var out []rune
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r)
		case r >= '0' && r <= '9':
			out = append(out, r)
		case r == '_' || r == ' ' || r == '-':
			out = append(out, '_')
		case r == 'á':
			out = append(out, 'a')
		case r == 'é':
			out = append(out, 'e')
		case r == 'í':
			out = append(out, 'i')
		case r == 'ó':
			out = append(out, 'o')
		case r == 'ú' || r == 'ü':
			out = append(out, 'u')
		case r == 'ñ':
			out = append(out, 'n')
		}
	}
	if len(out) == 0 {
		return "x"
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = append([]rune{'x', '_'}, out...)
	}
	return string(out)
}

func ensureDirs() {
	_ = os.MkdirAll("prolog", 0755)
	_ = os.MkdirAll("rpa_reports", 0755)
}

//
// ======== Generación del Prolog (.pl) ========
//

func buildPL(k Knowledge) string {
	var b strings.Builder

	// Síntomas
	for _, s := range k.Symptoms {
		b.WriteString(fmt.Sprintf("sintoma(%s).\n", atomize(s.Name)))
	}
	b.WriteString("\n")

	// Severidades
	b.WriteString("peso_severidad(leve,1).\n")
	b.WriteString("peso_severidad(moderado,2).\n")
	b.WriteString("peso_severidad(severo,3).\n\n")

	// Enfermedades
	for _, d := range k.Diseases {
		b.WriteString(fmt.Sprintf("enfermedad(%s, tipo(%s), sistema(%s)).\n",
			atomize(d.Name), atomize(d.Tipo), atomize(d.Sistema)))
	}
	b.WriteString("\n")

	// Características
	for _, d := range k.Diseases {
		for _, c := range d.Caracteristicas {
			b.WriteString(fmt.Sprintf("caracteriza(%s, %s, %d).\n",
				atomize(d.Name), atomize(c.Symptom), clamp(c.Peso, 1, 3)))
		}
	}
	b.WriteString("\n")

	// Medicamentos que tratan
	for _, m := range k.Meds {
		var ts []string
		for _, t := range m.Treats {
			ts = append(ts, atomize(t))
		}
		b.WriteString(fmt.Sprintf("trata(%s, [%s]).\n", atomize(m.Name), strings.Join(ts, ", ")))
	}
	b.WriteString("\n")

	// Contraindicaciones
	for _, ca := range k.ContraAlergias {
		b.WriteString(fmt.Sprintf("contraindicado_por_alergia(%s, %s).\n",
			atomize(ca.Med), atomize(ca.Alergia)))
	}
	for _, cc := range k.ContraCronicos {
		b.WriteString(fmt.Sprintf("contraindicado_por_cronico(%s, %s).\n",
			atomize(cc.Med), atomize(cc.Cronico)))
	}

	// Reglas y auxiliares (sin \+)
	b.WriteString(`
% ==== Auxiliares de listas ====
member(E,[E|_]).
member(E,[_|T]):-member(E,T).

sum_list([],0).
sum_list([H|T],S):-sum_list(T,S1),S is H+S1.

length([],0).
length([_|T],N):-length(T,N1),N is N1+1.

append([],L,L).
append([H|T],L,[H|R]):-append(T,L,R).

% ==== Afinidad ====
afinidad(Enf,Sv,Afin,Regs):-
  findall(W,(member((S,Sev),Sv),caracteriza(Enf,S,Pw),peso_severidad(Sev,Pv),W is Pw*Pv),Pesos),
  sum_list(Pesos,Suma),
  findall(Pmax,caracteriza(Enf,_,Pmax),Pmaxs),
  length(Pmaxs,N),(N=:=0->Max is 1; Max is 9*N),
  Raw is Suma/Max,
  Afin is round(Raw*100),
  findall(rule(caracteriza(Enf,S,Pw),severidad(S,Sev)),
          (member((S,Sev),Sv),caracteriza(Enf,S,Pw)),Regs).

% ==== Medicamento seguro (sin negación \+) ====
medicamento_seguro(Enf,Als,Crs,Med):-
  trata(Med,Ens), member(Enf,Ens),
  no_contra_alergias(Med, Als),
  no_contra_cronicos(Med, Crs).

no_contra_alergias(_, []).
no_contra_alergias(Med, [A|T]):- contraindicado_por_alergia(Med, A), !, fail.
no_contra_alergias(Med, [_|T]):- no_contra_alergias(Med, T).

no_contra_cronicos(_, []).
no_contra_cronicos(Med, [C|T]):- contraindicado_por_cronico(Med, C), !, fail.
no_contra_cronicos(Med, [_|T]):- no_contra_cronicos(Med, T).

% ==== Urgencia ====
nivel_urgencia(Enf,Sv,U):-
  ( Enf=influenza, member((fiebre,severo),Sv) -> U='Consulta médica inmediata sugerida'
  ; Enf=influenza, member((fiebre,moderado),Sv) -> U='Observación recomendada'
  ; Enf=migrana, member((dolor_cabeza,severo),Sv) -> U='Observación recomendada'
  ; U='Posible automanejo'
  ).

% ==== Consulta principal y ordenamiento ====
consulta(Sv,Als,Crs,Ordenado):-
  findall(res(Enf,A,Med,U),
    ( enfermedad(Enf,_,_),
      afinidad(Enf,Sv,A,_),
      A>0,
      (medicamento_seguro(Enf,Als,Crs,Med)->true;Med=ninguno),
      nivel_urgencia(Enf,Sv,U)
    ),Res),
  ordenar_por_afinidad(Res,Ordenado).

consulta_item(Sv,Als,Crs,Enf,A,Med,U):-
  consulta(Sv,Als,Crs,Ord),
  member(res(Enf,A,Med,U),Ord).

ordenar_por_afinidad([],[]).
ordenar_por_afinidad([H|T],S):-
  particionar_por_afinidad(H,T,May,Men),
  ordenar_por_afinidad(May,Sm),
  ordenar_por_afinidad(Men,Sn),
  append(Sm,[H|Sn],S).

afin_de(res(_,A,_,_),A).
particionar_por_afinidad(_,[],[],[]).
particionar_por_afinidad(P,[X|Xs],[X|May],Men):-afin_de(X,Ax),afin_de(P,Ap),Ax>=Ap,!,particionar_por_afinidad(P,Xs,May,Men).
particionar_por_afinidad(P,[X|Xs],May,[X|Men]):-particionar_por_afinidad(P,Xs,May,Men).
`)

	return b.String()
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

//
// ======== KB por defecto ========
//

func defaultKB() Knowledge {
	return Knowledge{
		Symptoms: []Symptom{
			{Name: "fiebre"},
			{Name: "tos"},
			{Name: "dolor_garganta"},
			{Name: "dolor_cabeza"},
			{Name: "fatiga"},
		},
		Diseases: []Disease{
			{
				Name:    "resfriado_comun",
				Tipo:    "viral",
				Sistema: "respiratorio",
				Caracteristicas: []Caract{
					{Symptom: "tos", Peso: 2},
					{Symptom: "dolor_garganta", Peso: 2},
					{Symptom: "fiebre", Peso: 1},
				},
			},
			{
				Name:    "influenza",
				Tipo:    "viral",
				Sistema: "respiratorio",
				Caracteristicas: []Caract{
					{Symptom: "fiebre", Peso: 3},
					{Symptom: "tos", Peso: 2},
					{Symptom: "fatiga", Peso: 2},
				},
			},
			{
				Name:    "migrana",
				Tipo:    "neurologico",
				Sistema: "nervioso",
				Caracteristicas: []Caract{
					{Symptom: "dolor_cabeza", Peso: 3},
					{Symptom: "fatiga", Peso: 1},
				},
			},
		},
		Meds: []Medication{
			{Name: "paracetamol", Treats: []string{"resfriado_comun", "influenza", "migrana"}},
			{Name: "ibuprofeno", Treats: []string{"resfriado_comun", "migrana"}},
			{Name: "oseltamivir", Treats: []string{"influenza"}},
			{Name: "jarabe_dextrometorfano", Treats: []string{"resfriado_comun"}},
		},
		ContraAlergias: []ContraAlergia{
			{Med: "ibuprofeno", Alergia: "aines"},
			{Med: "oseltamivir", Alergia: "oseltamivir_alergia"},
		},
		ContraCronicos: []ContraCronico{
			{Med: "ibuprofeno", Cronico: "hipertension_no_controlada"},
		},
	}
}

//
// ======== RPA ========
//

type rpaDisease struct {
	Name, Tipo, Sistema, Descripcion string
	Sintomas                         map[string]int // fiebre:3
	Contra                           []string       // medicamentos contraindicados (marcamos como alergia desconocida)
	Trata                            []string
}

type rpaParsed struct {
	Items []rpaDisease
}

func parseRPAFile(text string) rpaParsed {
	blocks := splitRPA(text)
	var items []rpaDisease
	for _, bl := range blocks {
		if strings.TrimSpace(bl) == "" {
			continue
		}
		d := rpaDisease{Sintomas: map[string]int{}}
		lines := strings.Split(bl, "\n")
		for _, ln := range lines {
			ln = strings.TrimSpace(ln)
			if ln == "" {
				continue
			}
			col := strings.Index(ln, ":")
			if col < 0 {
				continue
			}
			key := strings.TrimSpace(strings.ToLower(ln[:col]))
			val := strings.TrimSpace(ln[col+1:])
			switch key {
			case "nombre":
				d.Name = atomize(val)
			case "tipo":
				d.Tipo = atomize(val)
			case "sistema":
				d.Sistema = atomize(val)
			case "descripcion":
				d.Descripcion = val
			case "sintomas":
				for _, p := range strings.Split(val, ",") {
					p = strings.TrimSpace(p)
					if p == "" {
						continue
					}
					kv := strings.Split(p, ":")
					s := atomize(strings.TrimSpace(kv[0]))
					w := 1
					if len(kv) > 1 {
						fmt.Sscanf(kv[1], "%d", &w)
					}
					if w < 1 {
						w = 1
					}
					if w > 3 {
						w = 3
					}
					d.Sintomas[s] = w
				}
			case "contraindicados":
				d.Contra = parseCSVAtoms(val)
			case "trata":
				d.Trata = parseCSVAtoms(val)
			}
		}
		if d.Name != "" {
			items = append(items, d)
		}
	}
	return rpaParsed{Items: items}
}

func splitRPA(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.Split(text, "---")
}

func parseCSVAtoms(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, atomize(p))
	}
	return out
}

func applyParsedToKB(k *Knowledge, p rpaParsed) {
	for _, it := range p.Items {
		// sintomas nuevos
		for s := range it.Sintomas {
			if !hasSym(k.Symptoms, s) {
				k.Symptoms = append(k.Symptoms, Symptom{Name: s})
			}
		}
		// enfermedad (actualiza o inserta)
		upd := false
		for i := range k.Diseases {
			if k.Diseases[i].Name == it.Name {
				k.Diseases[i].Tipo = it.Tipo
				k.Diseases[i].Sistema = it.Sistema
				k.Diseases[i].Descripcion = it.Descripcion
				k.Diseases[i].Caracteristicas = nil
				for s, w := range it.Sintomas {
					k.Diseases[i].Caracteristicas = append(k.Diseases[i].Caracteristicas, Caract{Symptom: s, Peso: w})
				}
				upd = true
				break
			}
		}
		if !upd {
			var car []Caract
			for s, w := range it.Sintomas {
				car = append(car, Caract{Symptom: s, Peso: w})
			}
			k.Diseases = append(k.Diseases, Disease{
				Name: it.Name, Tipo: it.Tipo, Sistema: it.Sistema,
				Descripcion: it.Descripcion, Caracteristicas: car,
			})
		}
		// contraindicados -> marcamos como alergia "desconocida" para registrar el vínculo
		for _, m := range it.Contra {
			k.ContraAlergias = append(k.ContraAlergias, ContraAlergia{Med: m, Alergia: "desconocida"})
		}
		// trata
		for _, m := range it.Trata {
			found := false
			for i := range k.Meds {
				if k.Meds[i].Name == m {
					if !contains(k.Meds[i].Treats, it.Name) {
						k.Meds[i].Treats = append(k.Meds[i].Treats, it.Name)
					}
					found = true
					break
				}
			}
			if !found {
				k.Meds = append(k.Meds, Medication{Name: m, Treats: []string{it.Name}})
			}
		}
	}
}

func hasSym(list []Symptom, x string) bool {
	for _, s := range list {
		if s.Name == x {
			return true
		}
	}
	return false
}

func contains(list []string, x string) bool {
	for _, s := range list {
		if s == x {
			return true
		}
	}
	return false
}

func buildRPAReport(p rpaParsed) string {
	var b strings.Builder
	ts := time.Now().Format("2006-01-02 15:04:05")
	b.WriteString("MediLogic RPA – Informe de carga\n")
	b.WriteString("Fecha: " + ts + "\n\n")
	for _, it := range p.Items {
		b.WriteString(fmt.Sprintf("- Enfermedad: %s (tipo=%s, sistema=%s)\n", it.Name, it.Tipo, it.Sistema))
		b.WriteString("  Síntomas: ")
		var ss []string
		for k, v := range it.Sintomas {
			ss = append(ss, fmt.Sprintf("%s:%d", k, v))
		}
		b.WriteString(strings.Join(ss, ", "))
		b.WriteString("\n")
		if len(it.Trata) > 0 {
			b.WriteString("  Trata: " + strings.Join(it.Trata, ", ") + "\n")
		}
		if len(it.Contra) > 0 {
			b.WriteString("  Contraindicados: " + strings.Join(it.Contra, ", ") + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func deliverReport(report string) error {
	// SMTP opcional (variables de entorno):
	// SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, SMTP_FROM, ADMIN_EMAILS
	host := os.Getenv("SMTP_HOST")
	port := getenv("SMTP_PORT", "587")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")
	rcpts := strings.Split(os.Getenv("ADMIN_EMAILS"), ",")
	if host == "" || user == "" || pass == "" || from == "" || len(rcpts) == 0 || strings.TrimSpace(rcpts[0]) == "" {
		return fmt.Errorf("SMTP no configurado")
	}

	addr := host + ":" + port
	auth := smtp.PlainAuth("", user, pass, host)

	msg := bytes.Buffer{}
	msg.WriteString("To: " + strings.Join(rcpts, ",") + "\r\n")
	msg.WriteString("Subject: MediLogic RPA – Informe de cambios\r\n")
	msg.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n")
	msg.WriteString(report)

	return smtp.SendMail(addr, auth, from, rcpts, msg.Bytes())
}

func saveReportToDisk(report string) {
	fn := filepath.Join("rpa_reports", "rpa_"+time.Now().Format("20060102_150405")+".txt")
	_ = os.WriteFile(fn, []byte(report), 0644)
	logp("Informe RPA guardado en %s", fn)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
