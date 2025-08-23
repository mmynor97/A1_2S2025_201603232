# MediLogic (Entrega 1 – Tarea 1)

Sistema experto preliminar para apoyo diagnóstico, basado en Prolog (Ichiban) ejecutado desde Go.  
**No sustituye** atención médica profesional.

## Requisitos
- Go 1.22+
- (Opcional) Un servidor estático para `/frontend` (por ejemplo, VSCode Live Server)

## Estructura
- `backend/` servicio HTTP y base Prolog `prolog/medi_logic.pl`
- `frontend/` páginas HTML minimalistas (paciente/admin)
- `docs/` manual de usuario y técnico (borradores)
- `LICENSE` MIT

## Ejecutar
```bash
cd backend
go mod tidy
go run main.go
