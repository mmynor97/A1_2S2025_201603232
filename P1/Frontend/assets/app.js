// frontend/assets/app.js
(function () {
  "use strict";

  // === Config ===
  const BASE =
    (window.MEDI_CONFIG && window.MEDI_CONFIG.backendBaseUrl) ||
    "http://localhost:8080";

  // === Elementos base esperados en paciente.html ===
  const form = document.getElementById("form");
  if (!form) {
    console.warn("No se encontró #form. Revisa tu paciente.html");
    return;
  }
  const resultEl = ensureEl("#result");        // contenedor de resultados
  const toolbarEl = ensureToolbar();           // botones extra (PDF/Historial)
  const btnLimpiar = document.getElementById("limpiar");

  // === Eventos ===
  form.addEventListener("submit", onAnalyze);
  if (btnLimpiar) btnLimpiar.addEventListener("click", onClear);

  // Inyecta eventos de la toolbar (si no tenías botones en el HTML, los creo)
  toolbarEl.btnPDF.addEventListener("click", onDownloadPDF);
  toolbarEl.btnHist.addEventListener("click", toggleHistory);
  toolbarEl.btnClearHist.addEventListener("click", clearHistory);

  // Renderiza historial inicial (si existía)
  renderHistory();

  // === Lógica principal ===
  async function onAnalyze(e) {
    e.preventDefault();

    // Síntomas seleccionados
    const checked = Array.from(
      document.querySelectorAll('input[name="sym"]:checked')
    ).map((i) => i.value);

    if (!checked.length) {
      resultEl.innerHTML = info("Selecciona al menos un síntoma.");
      return;
    }

    const sintomas = checked.map((s) => ({
      nombre: s,
      severidad: document.querySelector(`select[name="sev_${s}"]`).value,
    }));

    // Alergias y crónicos
    const alergias = csv("#alergias");
    const cronicos = csv("#cronicos");

    const payload = { sintomas, alergias, cronicos };

    // UI: loading
    resultEl.innerHTML = `<div style="padding:10px;border:1px dashed #e5e7eb;border-radius:8px">Analizando...</div>`;

    try {
      const r = await fetch(BASE + "/analyze", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (!r.ok) {
        const t = await safeText(r);
        throw new Error(t || `Error ${r.status}`);
      }
      const data = await r.json();
      const rows = data.resultados || [];

      renderResultados(rows);
      saveHistory({ input: payload, output: rows });
    } catch (err) {
      resultEl.innerHTML = error(err.message || String(err));
    }
  }

  function onClear() {
    form.reset();
    resultEl.innerHTML = "";
  }

  // === Render de resultados (tabla + gráfico) ===
  function renderResultados(rows) {
    if (!rows.length) {
      resultEl.innerHTML =
        `<p class="muted">Sin coincidencias con las reglas actuales.</p>`;
      return;
    }

    // Tabla
    let html = `
      <section id="report-root">
        <h3 style="margin:8px 0 12px">Resultados</h3>
        <table style="width:100%;border-collapse:collapse">
          <thead>
            <tr>
              <th style="border:1px solid #e5e7eb;padding:8px;text-align:left">Enfermedad</th>
              <th style="border:1px solid #e5e7eb;padding:8px;text-align:left">Afinidad (%)</th>
              <th style="border:1px solid #e5e7eb;padding:8px;text-align:left">Medicamento sugerido</th>
              <th style="border:1px solid #e5e7eb;padding:8px;text-align:left">Urgencia</th>
            </tr>
          </thead>
          <tbody>
    `;
    for (const r of rows) {
      const enf = esc(r.enfermedad ?? JSON.stringify(r));
      const af = asNumber(r.afinidad);
      const med = esc(r.medicamento ?? "ninguno");
      const urg = esc(r.urgencia ?? "Observación recomendada");
      html += `
        <tr>
          <td style="border:1px solid #e5e7eb;padding:8px">${enf}</td>
          <td style="border:1px solid #e5e7eb;padding:8px">${af}</td>
          <td style="border:1px solid #e5e7eb;padding:8px">${med}</td>
          <td style="border:1px solid #e5e7eb;padding:8px"><span class="badge">${urg}</span></td>
        </tr>`;
    }
    html += `</tbody></table>`;

    // Gráfico de barras (SVG)
    html += `<div style="margin-top:14px">${renderAffinityChart(rows)}</div>`;

    // Metadatos del informe (para PDF)
    const meta = buildReportMeta();
    html += `
      <div id="report-meta" style="margin-top:14px;font-size:12px;color:#6b7280">
        <div><strong>Fecha:</strong> ${esc(meta.fecha)}</div>
        <div><strong>Fuente:</strong> MediLogic (herramienta de apoyo diagnóstico)</div>
        <div><strong>Nota:</strong> No sustituye consulta médica profesional.</div>
      </div>
      </section>
    `;

    resultEl.innerHTML = html;
  }

  // === Gráfico SVG de afinidad (simple) ===
  function renderAffinityChart(rows) {
    const W = 720, H = 260, P = 28; // ancho, alto, padding
    const data = rows.map((r) => ({
      name: String(r.enfermedad ?? ""),
      val: Math.max(0, Math.min(100, asNumber(r.afinidad))),
    }));
    const max = 100;
    const bw = Math.max(10, (W - P * 2) / Math.max(1, data.length) - 10);

    let svg = `<svg viewBox="0 0 ${W} ${H}" role="img" aria-label="Afinidad por enfermedad" style="width:100%;max-width:800px;border:1px solid #f1f5f9;border-radius:12px;background:#fff">`;

    // Ejes
    svg += `<line x1="${P}" y1="${H - P}" x2="${W - P}" y2="${H - P}" stroke="#e5e7eb"/>`;
    svg += `<line x1="${P}" y1="${P}" x2="${P}" y2="${H - P}" stroke="#e5e7eb"/>`;

    // Barras
    data.forEach((d, i) => {
      const x = P + 10 + i * (bw + 10);
      const h = ((H - P * 2) * d.val) / max;
      const y = H - P - h;
      svg += `<rect x="${x}" y="${y}" width="${bw}" height="${h}" rx="4" ry="4" fill="#1f7ae0" opacity="0.9"></rect>`;
      // Etiquetas
      svg += `<text x="${x + bw / 2}" y="${H - P + 16}" font-size="11" text-anchor="middle" fill="#334155">${esc(short(d.name, 12))}</text>`;
      svg += `<text x="${x + bw / 2}" y="${y - 6}" font-size="11" text-anchor="middle" fill="#0f172a">${d.val}</text>`;
    });

    // Título
    svg += `<text x="${P}" y="${P - 8}" font-size="12" fill="#334155">Afinidad (%)</text>`;

    svg += `</svg>`;
    return svg;
  }

  // === Historial (solo sesión activa) ===
  function saveHistory(entry) {
    try {
      const key = "medilogic_history_session";
      const now = new Date().toISOString();
      const hist = JSON.parse(sessionStorage.getItem(key) || "[]");
      hist.unshift({ at: now, ...entry });
      while (hist.length > 10) hist.pop(); // máximo 10
      sessionStorage.setItem(key, JSON.stringify(hist));
      renderHistory();
    } catch {
      /* ignore */
    }
  }

  function renderHistory() {
    const box = ensureHistoryBox();
    const key = "medilogic_history_session";
    const hist = safeJSON(sessionStorage.getItem(key)) || [];

    if (!hist.length) {
      box.innerHTML =
        `<div class="muted" style="font-size:13px">Sin diagnósticos previos en esta sesión.</div>`;
      return;
    }

    let html = `<ul style="list-style:none;padding:0;margin:0;display:grid;gap:8px">`;
    hist.forEach((h, idx) => {
      const titulo = new Date(h.at).toLocaleString();
      const resumen =
        (h.output && h.output[0] && h.output[0].enfermedad) ?
          `Top: ${esc(h.output[0].enfermedad)} (${asNumber(h.output[0].afinidad)}%)` :
          `Sin resultados`;
      html += `
        <li style="border:1px solid #e5e7eb;border-radius:8px;padding:8px">
          <div style="font-weight:600">${esc(titulo)}</div>
          <div style="color:#64748b;font-size:13px">${resumen}</div>
          <div style="margin-top:6px;display:flex;gap:6px">
            <button data-idx="${idx}" class="btn small" data-action="ver">Ver</button>
            <button data-idx="${idx}" class="btn small ghost" data-action="reusar">Reusar entrada</button>
          </div>
        </li>`;
    });
    html += `</ul>`;

    box.innerHTML = html;

    // acciones
    box.querySelectorAll("button").forEach((b) => {
      b.addEventListener("click", () => {
        const idx = parseInt(b.getAttribute("data-idx"), 10);
        const action = b.getAttribute("data-action");
        const hist = safeJSON(sessionStorage.getItem(key)) || [];
        const item = hist[idx];
        if (!item) return;

        if (action === "ver") {
          renderResultados(item.output || []);
          window.scrollTo({ top: resultEl.offsetTop - 12, behavior: "smooth" });
        } else if (action === "reusar") {
          fillForm(item.input || {});
          window.scrollTo({ top: form.offsetTop - 12, behavior: "smooth" });
        }
      });
    });
  }

  function fillForm(input) {
    // limpia
    form.reset();
    // síntomas
    if (input.sintomas && Array.isArray(input.sintomas)) {
      input.sintomas.forEach((s) => {
        const cb = document.querySelector(`input[name="sym"][value="${cssq(s.nombre)}"]`);
        if (cb) {
          cb.checked = true;
          const sel = document.querySelector(`select[name="sev_${cssq(s.nombre)}"]`);
          if (sel && s.severidad) sel.value = s.severidad;
        }
      });
    }
    // alergias, crónicos
    const al = document.getElementById("alergias");
    const cr = document.getElementById("cronicos");
    if (al && input.alergias) al.value = (input.alergias || []).join(", ");
    if (cr && input.cronicos) cr.value = (input.cronicos || []).join(", ");
  }

  function toggleHistory() {
  const wrap = document.getElementById("history-wrap");
  if (!wrap) return;
  const isHidden = wrap.style.display === "none" || wrap.classList.contains("hidden");
  if (isHidden) {
    wrap.style.display = "";
    wrap.classList.remove("hidden");
  } else {
    wrap.style.display = "none";
    wrap.classList.add("hidden");
  }
}


  function clearHistory() {
    sessionStorage.removeItem("medilogic_history_session");
    renderHistory();
  }

  // === PDF / Impresión ===
  function onDownloadPDF() {
    // Crea una ventana imprimible con el contenido del informe
    const report = document.getElementById("report-root");
    if (!report) {
      alert("No hay informe para imprimir. Ejecuta un análisis primero.");
      return;
    }
    const meta = document.getElementById("report-meta")?.outerHTML || "";
    const html = `
<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<title>MediLogic – Informe</title>
<style>
  body{font-family:system-ui,Segoe UI,Arial;margin:24px;color:#0f172a}
  h1{margin:0 0 8px}
  .muted{color:#64748b}
  table{width:100%;border-collapse:collapse;margin-top:12px}
  th,td{border:1px solid #e5e7eb;padding:8px;text-align:left;font-size:14px}
  @media print {
    .no-print{display:none}
  }
</style>
</head>
<body>
  <h1>MediLogic – Informe</h1>
  <div class="muted">Herramienta de apoyo diagnóstico (no sustituye consulta médica).</div>
  ${report.outerHTML}
  ${meta}
  <div class="no-print" style="margin-top:16px"><button onclick="window.print()">Imprimir</button></div>
</body>
</html>`;
    const w = window.open("", "_blank");
    if (!w) return;
    w.document.open();
    w.document.write(html);
    w.document.close();
    // Espera un frame para que calcule layout y abre diálogo de imprimir:
    setTimeout(() => w.print(), 300);
  }

  // === Helpers de DOM/UI ===
  function ensureEl(selector) {
    let el = document.querySelector(selector);
    if (!el) {
      // crea el contenedor si no existía
      el = document.createElement("div");
      el.id = selector.replace("#", "");
      form.insertAdjacentElement("afterend", el);
    }
    return el;
  }

  function ensureToolbar() {
    // intenta encontrar un contenedor .actions; si no, lo crea
    let actions = document.querySelector(".actions");
    if (!actions) {
      actions = document.createElement("div");
      actions.className = "actions";
      form.appendChild(actions);
    }
    // botón PDF
    const btnPDF = document.createElement("button");
    btnPDF.type = "button";
    btnPDF.className = "btn ghost";
    btnPDF.textContent = "Descargar PDF";
    actions.appendChild(btnPDF);

    // botón historial
    const btnHist = document.createElement("button");
    btnHist.type = "button";
    btnHist.className = "btn ghost";
    btnHist.textContent = "Historial (sesión)";
    actions.appendChild(btnHist);

    // botón limpiar historial
    const btnClearHist = document.createElement("button");
    btnClearHist.type = "button";
    btnClearHist.className = "btn ghost";
    btnClearHist.textContent = "Limpiar historial";
    actions.appendChild(btnClearHist);

    // contenedor de historial
    const wrap = document.createElement("section");
    wrap.id = "history-wrap";
    wrap.className = "hidden";
    wrap.style.marginTop = "14px";
    wrap.innerHTML = `<h3>Historial de esta sesión</h3><div id="history-box"></div>`;
    resultEl.insertAdjacentElement("beforebegin", wrap);

    return { btnPDF, btnHist, btnClearHist, historyWrap: wrap };
  }

  function ensureHistoryBox() {
  let wrap = document.getElementById("history-wrap");
  if (!wrap) {
    wrap = document.createElement("section");
    wrap.id = "history-wrap";
    wrap.className = "hidden";     
    wrap.style.display = "none";
    wrap.style.marginTop = "14px";
    wrap.innerHTML = `<h3>Historial de esta sesión</h3>`;
    const res = ensureResultEl();
    res.insertAdjacentElement("beforebegin", wrap);
  }

  let box = document.getElementById("history-box");
  if (!box) {
    box = document.createElement("div");
    box.id = "history-box";
    wrap.appendChild(box);
  }
  return box;
}


  // === Utilidades varias ===
  function csv(sel) {
    const el = document.querySelector(sel);
    if (!el) return [];
    return el.value.split(",").map((s) => s.trim()).filter(Boolean);
  }

  function esc(s) {
    return String(s).replace(/[&<>"']/g, (m) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[m])
    );
  }

  function info(msg) {
    return `<div style="background:#fffbeb;border:1px solid #fde68a;padding:10px;border-radius:8px">${esc(
      msg
    )}</div>`;
  }

  function error(msg) {
    return `<div style="background:#fef2f2;border:1px solid #fecaca;padding:10px;border-radius:8px"><strong>Error:</strong> ${esc(
      msg
    )}</div>`;
  }

  function asNumber(v) {
    const n = typeof v === "number" ? v : parseFloat(String(v).replace(",", "."));
    return Number.isFinite(n) ? Math.round(n) : 0;
  }

  function short(s, n) {
    s = String(s || "");
    return s.length > n ? s.slice(0, n - 1) + "…" : s;
  }

  function cssq(s) {
    // para usar en selectores (valor de checkbox), por si tuviera caracteres especiales
    return String(s).replace(/"/g, '\\"');
  }

  async function safeText(r) {
    try { return await r.text(); } catch { return ""; }
  }

  function buildReportMeta() {
    const now = new Date();
    return { fecha: now.toLocaleString() };
  }
})();
