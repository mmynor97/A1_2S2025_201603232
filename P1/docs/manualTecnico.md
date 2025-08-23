# Manual Técnico – MediLogic (Borrador)

## 1. Arquitectura
- **Frontend**: HTML/CSS/JS plano.
- **Backend**: Go + Ichiban Prolog.
- **Conocimiento**: `prolog/medi_logic.pl`.

## 2. Motor lógico
- Reglas en Prolog:
  - `afinidad/4` calcula %.
  - `medicamento_seguro/4` filtra contraindicaciones.
  - `nivel_urgencia/3` asigna urgencia.
  - `consulta/4` orquesta la respuesta.

## 3. Endpoints
- `GET /health` → "ok"
- `GET /admin/export` → descarga `.pl` actual.
- `POST /analyze` → `{"sintomas":[{"nombre":"fiebre","severidad":"severo"}], "alergias":["aines"], "cronicos":[]}`

## 4. Compilación
```bash
cd backend
go mod tidy
go run main.go
