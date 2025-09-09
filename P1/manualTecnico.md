# Manual técnico – MediLogic (Go + Prolog)

Este documento describe la arquitectura, instalación, ejecución, APIs, estructura de la base de conocimiento, módulo administrativo, RPA, despliegue y solución de problemas del sistema **MediLogic** desarrollado para la Tarea 1.

---
## 1. Arquitectura General

**Objetivo:** Orientación diagnóstica preliminar basada en una base de conocimiento Prolog ejecutada desde Go.

### Componentes

* **Backend (Go)**
    * Motor Prolog: [github.com/ichiban/prolog](https://github.com/ichiban/prolog).
    * Endpoints REST: `/analyze`, `/admin/*`.
    * Generador de `.pl` desde una KB en JSON.
    * RPA: ingesta de texto plano → actualiza KB, regenera `.pl`, recarga motor.
    * (Opcional) envío de informe por correo vía SMTP.

* **Base de Conocimiento (Prolog)**
    * **Hechos:** `sintoma/1`, `enfermedad/3`, `caracteriza/3`, `trata/2`, `contraindicaciones`.
    * **Reglas:** `afinidad/4`, `medicamento_seguro/4` (sin negación `\+`), `nivel_urgencia/3`, `consulta/4`, `consulta_item/7` y auxiliares.

* **Frontend (Estático)**
    * `paciente.html` + `assets/app.js`: formularios de síntomas/alergias/crónicos, invoca `/analyze`, dibuja tabla y gráfico SVG, historial en `sessionStorage`, descarga PDF.
    * `admin.html`: panel protegido por token; edita KB (JSON), exporta/sube `.pl`, ejecuta RPA.

---

## 2. Estructura del Repositorio

P1/
├─ backend/
│  ├─ main.go                 # servidor Go + integración Prolog
│  ├─ prolog/
│  │  └─ medi_logic.pl        # KB activa (generada o subida)
│  └─ rpa_reports/            # informes de RPA (si no hay SMTP)
├─ frontend/
│  ├─ paciente.html
│  ├─ admin.html
│  └─ assets/
│     ├─ app.js
│     └─ styles.css (opcional)
└─ .gitignore / LICENSE(MIT) / README.md


---

## 3. Requisitos y Dependencias

* **Go** ≥ 1.20 (Windows x64 probado).
* **Módulos Go:** `github.com/ichiban/prolog`.
* Navegador moderno (Chrome/Edge/Firefox).
* **Opcional SMTP** (para enviar informes RPA):
    `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`, `ADMIN_EMAILS`.

---

## 4. Instalación y Ejecución (Windows)

### Backend

```bash
cd P1\backend
go mod init medi-logic           # solo una vez
go get [github.com/ichiban/prolog@latest](https://github.com/ichiban/prolog@latest)
go mod tidy
go run .
# → MediLogic backend en http://localhost:8080

```
Frontend
Abre P1/frontend/paciente.html y P1/frontend/admin.html con doble clic, o sírvelos estáticamente (ej., python -m http.server 5500).

## 5. Endpoints

### 5.1 POST /analyze

```json
{
  "sintomas": [
    {"nombre":"fiebre","severidad":"severo"},
    {"nombre":"tos","severidad":"leve"}
  ],
  "alergias": ["aines","oseltamivir_alergia"],
  "cronicos": ["hipertension_no_controlada"]
}
```
Salida JSON:

```json
{
  "resultados": [
    {
      "enfermedad": "influenza",
      "afinidad": 78,
      "medicamento": "paracetamol",
      "urgencia": "Consulta médica inmediata sugerida"
    },
    {
      "enfermedad": "resfriado_comun",
      "afinidad": 44,
      "medicamento": "jarabe_dextrometorfano",
      "urgencia": "Posible automanejo"
    }
  ]
}
```

- Ordenado descendente por afinidad. El medicamento sugerido filtra alergias y crónicos.

###  5.2 dministración

- GET /admin/export?token=ADMIN_TOKEN: Descarga el .pl activo.
- GET /admin/kb?token=ADMIN_TOKEN: Devuelve la KB en JSON.
- POST /admin/kb?token=ADMIN_TOKEN: Recibe KB JSON, regenera .pl y recarga Prolog.
- POST /admin/upload-pl?token=ADMIN_TOKEN: Sube un .pl, lo guarda y recarga el motor.
- POST /admin/rpa/ingest?token=ADMIN_TOKEN: Ingiere texto plano con bloques --- Actualiza KB, regenera .pl, recarga y emite informe

<b>Seguridad:</b> Cabecera X-Admin-Token: <token> o query ?token=<token>.

- Token por defecto: admin123 (cámbialo con env ADMIN_TOKEN).

- CORS: Abierto para * (útil en desarrollo).

## 6. Integración Go ↔ Prolog

Motor creado con prolog.New(nil, nil). Carga con vm.Exec(code).

- Consulta: consulta_item(Sv,Als,Crs,Enf,Afin,Med,Urg).

- Serialización de términos:

- Síntomas: [(fiebre,severo),(tos,leve)]

- Alergias/Crón: [aines, hipertension_no_controlada]

- Lectura de soluciones vía solutions.Scan(&rowStruct).

- Nota clave: Se evitó \+ (negación) en Prolog para compatibilidad. La regla de medicamento seguro usa recursión + corte/fallo.

```prolog
medicamento_seguro(Enf,Als,Crs,Med):-
  trata(Med,Ens), member(Enf,Ens),
  no_contra_alergias(Med, Als),
  no_contra_cronicos(Med, Crs).

no_contra_alergias(_, []).
no_contra_alergias(Med, [A|T]):- contraindicado_por_alergia(Med, A), !, fail.
no_contra_alergias(Med, [_|T]):- no_contra_alergias(Med, T).
```

## 7. Base de Conocimiento (Prolog)

Hechos base

- sintoma/1

- peso_severidad/2 (leve=1, moderado=2, severo=3)

- enfermedad/3

- caracteriza/3

- trata/2

- contraindicado_por_alergia/2

- contraindicado_por_cronico/2

Reglas clave

- afinidad/4 → calcula % normalizando por 3*3*N_sintomas.

- medicamento_seguro/4 → evita alergias/crónicos.

- nivel_urgencia/3 → heurística inicial.

- consulta/4 → lista de res(Enf,Afin,Med,Ur) ordenada.

- consulta_item/7 → iteración simple desde Go.

## 8. Frontend

### 8.1 Pacientes

- paciente.html: formulario con checkboxes de síntomas y selects de severidad; inputs de alergias y crónicos.

- assets/app.js:

1. Construye payload, llama a /analyze.

2. Renderiza tabla + gráfico SVG de afinidad.

3. Historial temporal (sessionStorage ≤ 10 entradas).

4. Descarga PDF.

5. Botones de Historial y Limpiar historial.

### 8.2 Administrador

- admin.html: panel con token, botones para Cargar/Guardar KB, Exportar/Subir .pl, Procesar RPA. Opera contra /admin/*.

## 9. RPA (Ingesta de Texto)

---
Nombre: sinusitis
Tipo: bacteriano
Sistema: respiratorio
Descripcion: texto libre
Sintomas: dolor_cabeza:2, fatiga:1, fiebre:2
Contraindicados: ibuprofeno
Trata: amoxicilina, paracetamol
---

Proceso backend:

1. Parseo → rpaParsed.

2. Actualización de KB (inserta/actualiza enfermedades, etc.).

3. Genera .pl y recarga motor.

4. Emite informe (SMTP o guarda en rpa_reports/).

## 10. Configuración.

- ADMIN_TOKEN (string) – token admin (default admin123).

- SMTP (ver arriba).

- Cambiar puerto: Edita ListenAndServe(":8080", nil) en el código y MEDI_CONFIG.backendBaseUrl en el frontend.

## 11. Pruebas rápidas.

Health

```bash
curl http://localhost:8080/health
```
Analyze (PowerShell)

```bash
$body = @{
  sintomas = @(
    @{ nombre="fiebre"; severidad="severo" },
    @{ nombre="tos";    severidad="leve"   }
  )
  alergias = @("aines")
  cronicos = @("hipertension_no_controlada")
} | ConvertTo-Json -Depth 5

Invoke-RestMethod -Method Post -Uri "http://localhost:8080/analyze" -ContentType "application/json" -Body $body
```

Export .pl

```bash
curl "http://localhost:8080/admin/export?token=admin123" -o medi_logic.pl
```

Subir .pl

```bash
Invoke-WebRequest -Method Post -Uri "http://localhost:8080/admin/upload-pl?token=admin123" `
  -InFile .\medi_logic.pl -ContentType "text/plain"
```

## 12. Errores frecuentes.

- error(existence_error(procedure,\+ /2), member/2): Usaste \+. Solución: reglas sin negación (ej., no_contra_* con corte/fallo).

-can't convert to term: <invalid reflect.Value>: Hacías vm.Exec(code, nil). Solución: vm.Exec(code).

- too many arguments in solutions.Scan: Scan recibe un struct. Solución: var row struct{...}; solutions.Scan(&row).

- CORS/404 desde HTML: Verifica que el backend esté en la URL configurada.

- Sin resultados: Revisa que los átomos estén en minúsculas con guión bajo (ej., dolor_cabeza).

## 13. Extensión y mantenimiento.

- Nuevas enfermedades: Usa el panel Admin, RPA o edita directamente el JSON.

-Nuevos síntomas: Agrégalos a la KB.

- Heurística de urgencia: Ajusta la regla nivel_urgencia/3 en .pl.

- Reglas adicionales: Crea nuevos predicados en .pl.

## 14. Despliegue binario

```bash
cd P1\backend
go build -o medilogic.exe .
# Ejecuta medilogic.exe (asegúrate de tener prolog/ y rpa_reports/ creados)
```

## 15. Licencia y Repsositorio.

Este proyecto está licenciado bajo los términos de la [MIT License](./LICENSE).