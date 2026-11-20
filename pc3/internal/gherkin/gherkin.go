// Package gherkin implementa un intérprete mínimo de archivos .feature
// (Característica/Regla/Antecedentes/Escenario/Esquema del escenario/Ejemplos),
// en español e inglés, usando solo la librería estándar. Sustituye a Cucumber/Godog
// para que el proyecto compile sin dependencias externas.
package gherkin

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Step struct {
	Keyword string
	Text    string
	Line    int
}

type Scenario struct {
	Name  string
	Rule  string
	Steps []Step
	Line  int
}

type Feature struct {
	File        string
	Name        string
	Description []string
	Background  []Step
	Scenarios   []Scenario
}

var (
	kwFeature    = []string{"Característica", "Caracteristica", "Funcionalidad", "Feature"}
	kwBackground = []string{"Antecedentes", "Fondo", "Background"}
	kwRule       = []string{"Regla", "Rule"}
	kwOutline    = []string{"Esquema del escenario", "Esquema del Escenario", "Scenario Outline"}
	kwScenario   = []string{"Escenario", "Ejemplo", "Scenario", "Example"}
	kwExamples   = []string{"Ejemplos", "Examples"}
	kwSteps      = []string{"Dado", "Cuando", "Entonces", "Y", "Pero", "Given", "When", "Then", "And", "But", "*"}
)

func structural(line string, kws []string) (string, bool) {
	for _, k := range kws {
		if strings.HasPrefix(line, k+":") {
			return strings.TrimSpace(line[len(k)+1:]), true
		}
	}
	return "", false
}

func stepLine(line string) (kw, text string, ok bool) {
	for _, k := range kwSteps {
		if strings.HasPrefix(line, k+" ") {
			return k, strings.TrimSpace(line[len(k)+1:]), true
		}
	}
	return "", "", false
}

func cells(line string) []string {
	line = strings.Trim(strings.TrimSpace(line), "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

type outline struct {
	sc      Scenario
	headers []string
	rows    [][]string
}

// Parse analiza el texto de un .feature.
func Parse(file, src string) (*Feature, error) {
	f := &Feature{File: file}
	const (
		sNone = iota
		sFeature
		sBackground
		sScenario
		sExamples
	)
	state := sNone
	rule := ""
	var cur *Scenario
	var out *outline
	order := []any{} // *Scenario | *outline, para conservar el orden

	flush := func() {
		if cur != nil {
			order = append(order, cur)
			cur = nil
		}
		if out != nil {
			order = append(order, out)
			out = nil
		}
	}

	for i, raw := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		n := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "@") {
			continue
		}
		if name, ok := structural(line, kwFeature); ok {
			f.Name, state = name, sFeature
			continue
		}
		if _, ok := structural(line, kwBackground); ok {
			flush()
			state = sBackground
			continue
		}
		if name, ok := structural(line, kwRule); ok {
			flush()
			rule, state = name, sNone
			continue
		}
		if name, ok := structural(line, kwOutline); ok {
			flush()
			out = &outline{sc: Scenario{Name: name, Rule: rule, Line: n}}
			state = sScenario
			continue
		}
		if name, ok := structural(line, kwScenario); ok {
			flush()
			cur = &Scenario{Name: name, Rule: rule, Line: n}
			state = sScenario
			continue
		}
		if _, ok := structural(line, kwExamples); ok {
			if out == nil {
				return nil, fmt.Errorf("%s:%d: Ejemplos sin Esquema del escenario", file, n)
			}
			state = sExamples
			out.headers = nil
			continue
		}
		if strings.HasPrefix(line, "|") {
			if state != sExamples || out == nil {
				return nil, fmt.Errorf("%s:%d: tabla fuera de Ejemplos", file, n)
			}
			if out.headers == nil {
				out.headers = cells(line)
			} else {
				row := cells(line)
				if len(row) != len(out.headers) {
					return nil, fmt.Errorf("%s:%d: la fila tiene %d celdas y hay %d columnas", file, n, len(row), len(out.headers))
				}
				out.rows = append(out.rows, append([]string{}, row...))
			}
			continue
		}
		if kw, text, ok := stepLine(line); ok && (state == sBackground || state == sScenario) {
			st := Step{kw, text, n}
			switch {
			case state == sBackground:
				f.Background = append(f.Background, st)
			case out != nil:
				out.sc.Steps = append(out.sc.Steps, st)
			case cur != nil:
				cur.Steps = append(cur.Steps, st)
			}
			continue
		}
		if state == sFeature {
			f.Description = append(f.Description, line)
			continue
		}
		return nil, fmt.Errorf("%s:%d: línea no reconocida: %q", file, n, line)
	}
	flush()

	for _, it := range order {
		switch v := it.(type) {
		case *Scenario:
			f.Scenarios = append(f.Scenarios, *v)
		case *outline:
			for k, row := range v.rows {
				sc := Scenario{Rule: v.sc.Rule, Line: v.sc.Line}
				var tag []string
				for j, h := range v.headers {
					tag = append(tag, h+"="+row[j])
				}
				sc.Name = fmt.Sprintf("%s #%d [%s]", v.sc.Name, k+1, strings.Join(tag, ", "))
				for _, st := range v.sc.Steps {
					t := st.Text
					for j, h := range v.headers {
						t = strings.ReplaceAll(t, "<"+h+">", row[j])
					}
					sc.Steps = append(sc.Steps, Step{st.Keyword, t, st.Line})
				}
				f.Scenarios = append(f.Scenarios, sc)
			}
		}
	}
	return f, nil
}

// LoadDir lee todos los *.feature de un directorio (orden alfabético).
func LoadDir(dir string) ([]*Feature, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.feature"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("no hay archivos .feature en %q", dir)
	}
	var fs []*Feature
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		f, err := Parse(p, string(b))
		if err != nil {
			return nil, err
		}
		fs = append(fs, f)
	}
	return fs, nil
}

// ---------------------------------------------------------------------------
// Ejecución
// ---------------------------------------------------------------------------

type stepDef[W any] struct {
	re *regexp.Regexp
	fn func(w *W, args []string) error
}

// Registry asocia expresiones regulares con funciones (step definitions).
type Registry[W any] struct{ defs []stepDef[W] }

// Step registra una definición. El patrón se compara contra el texto del paso
// (sin la palabra clave Dado/Cuando/Entonces/Y).
func (r *Registry[W]) Step(pattern string, fn func(w *W, args []string) error) {
	r.defs = append(r.defs, stepDef[W]{regexp.MustCompile(pattern), fn})
}

type StepResult struct {
	Step   Step
	Status string // ok | fallo | no definido | omitido
	Err    error
}

type ScenarioResult struct {
	Feature  string
	Scenario Scenario
	Steps    []StepResult
	Passed   bool
}

// Run ejecuta todos los escenarios de una feature con un World nuevo cada uno.
func (r *Registry[W]) Run(f *Feature, newWorld func() *W) []ScenarioResult {
	var res []ScenarioResult
	for _, sc := range f.Scenarios {
		w := newWorld()
		sr := ScenarioResult{Feature: f.Name, Scenario: sc, Passed: true}
		failed := false
		all := append(append([]Step{}, f.Background...), sc.Steps...)
		for _, st := range all {
			if failed {
				sr.Steps = append(sr.Steps, StepResult{Step: st, Status: "omitido"})
				continue
			}
			status, err := r.exec(w, st)
			sr.Steps = append(sr.Steps, StepResult{st, status, err})
			if status != "ok" {
				failed, sr.Passed = true, false
			}
		}
		res = append(res, sr)
	}
	return res
}

func (r *Registry[W]) exec(w *W, st Step) (string, error) {
	for _, d := range r.defs {
		if m := d.re.FindStringSubmatch(st.Text); m != nil {
			if err := d.fn(w, m[1:]); err != nil {
				return "fallo", err
			}
			return "ok", nil
		}
	}
	return "no definido", fmt.Errorf("paso sin definición: %q", st.Text)
}

// Report imprime los resultados con el estilo Given/When/Then y devuelve (ok, fallidos).
func Report(out io.Writer, results []ScenarioResult) (passed, failed int) {
	lastFeature, lastRule := "", ""
	for _, sr := range results {
		if sr.Feature != lastFeature {
			fmt.Fprintf(out, "\nCaracterística: %s\n", sr.Feature)
			lastFeature, lastRule = sr.Feature, ""
		}
		if sr.Scenario.Rule != "" && sr.Scenario.Rule != lastRule {
			fmt.Fprintf(out, "  Regla: %s\n", sr.Scenario.Rule)
			lastRule = sr.Scenario.Rule
		}
		mark := "✔"
		if !sr.Passed {
			mark = "✘"
			failed++
		} else {
			passed++
		}
		fmt.Fprintf(out, "    %s Escenario: %s\n", mark, sr.Scenario.Name)
		for _, s := range sr.Steps {
			icon := map[string]string{"ok": "✔", "fallo": "✘", "no definido": "?", "omitido": "-"}[s.Status]
			fmt.Fprintf(out, "        %s %-9s %s\n", icon, s.Step.Keyword, s.Step.Text)
			if s.Err != nil {
				fmt.Fprintf(out, "            -> %v\n", s.Err)
			}
		}
	}
	fmt.Fprintf(out, "\n%d escenarios (%d exitosos, %d fallidos)\n", passed+failed, passed, failed)
	return
}
