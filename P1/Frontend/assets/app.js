(function(){
  const form = document.getElementById('form');
  if (!form) return;

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    const checked = Array.from(document.querySelectorAll('input[name="sym"]:checked')).map(i => i.value);
    const sintomas = checked.map(s => ({
      nombre: s,
      severidad: document.querySelector(`select[name="sev_${s}"]`).value
    }));

    const alergias = document.getElementById('alergias').value.split(',').map(s=>s.trim()).filter(Boolean);
    const cronicos = document.getElementById('cronicos').value.split(',').map(s=>s.trim()).filter(Boolean);

    const payload = { sintomas, alergias, cronicos };

    const r = await fetch('http://localhost:8080/analyze', {
      method: 'POST',
      headers: { 'Content-Type':'application/json' },
      body: JSON.stringify(payload)
    });

    const data = await r.json();
    renderResultados(data.resultados || []);
  });

  function renderResultados(rows){
    const el = document.getElementById('result');
    if (!rows.length) {
      el.innerHTML = `<p>No se encontraron coincidencias con las reglas actuales.</p>`;
      return;
    }
    // Render tabla sencilla (afinidad, enfermedad, medicamento, urgencia)
    let html = `<h3>Resultados</h3>
    <table>
      <thead><tr><th>Enfermedad</th><th>Afinidad (%)</th><th>Medicamento sugerido</th><th>Urgencia</th></tr></thead>
      <tbody>`;
    for (const r of rows) {
      const enf = r.enfermedad || JSON.stringify(r);
      const af = r.afinidad ?? '-';
      const med = r.medicamento ?? 'ninguno';
      const urg = r.urgencia ?? '-';
      html += `<tr>
        <td>${enf}</td><td>${af}</td><td>${med}</td><td><span class="badge">${urg}</span></td>
      </tr>`;
    }
    html += `</tbody></table>
    <p class="muted">*Trazas de reglas disponibles en la consola del backend para Entrega 1.</p>`;
    el.innerHTML = html;
  }
})();