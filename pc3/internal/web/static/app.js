"use strict";
const $ = (id) => document.getElementById(id);
let token = null;

async function api(path, method, body) {
  const res = await fetch(path, {
    method: method || "GET",
    headers: Object.assign({ "Content-Type": "application/json" }, token ? { Authorization: "Bearer " + token } : {}),
    body: body ? JSON.stringify(body) : undefined,
  });
  let data = {};
  try { data = await res.json(); } catch (_) {}
  return { ok: res.ok, status: res.status, data };
}

function msg(el, text, cls) { el.textContent = text; el.className = "msg " + (cls || ""); }
const money = (v) => "S/ " + Number(v).toFixed(2);

function pintarCuenta(c) {
  $("titular").textContent = c.titular + " (" + c.id + ")";
  $("saldo").textContent = money(c.saldo);
  $("limite").textContent = money(c.retirado_hoy) + " / " + money(c.limite_diario);
}

async function cargarHistorial() {
  const r = await api("/api/transacciones");
  const tb = $("hist");
  tb.replaceChildren();
  if (!r.ok) return;
  $("json").textContent = JSON.stringify(r.data, null, 2);
  r.data.slice().reverse().forEach((t) => {
    const tr = document.createElement("tr");
    const celdas = [t.id, new Date(t.fecha).toLocaleString(), t.tipo, money(t.monto), null, money(t.saldo_despues), t.mensaje];
    celdas.forEach((v, i) => {
      const td = document.createElement("td");
      if (i === 4) {
        const s = document.createElement("span");
        s.className = "b " + t.estado; s.textContent = t.estado; td.appendChild(s);
      } else { td.textContent = v; if (i === 6) td.className = "det"; }
      tr.appendChild(td);
    });
    tb.appendChild(tr);
  });
}

$("btn-login").onclick = async () => {
  const r = await api("/api/login", "POST", { cuenta: $("cuenta").value.trim(), pin: $("pin").value });
  if (!r.ok) return msg($("login-msg"), r.data.error ? r.data.error.mensaje : "Error", "err");
  token = r.data.token;
  $("login").hidden = true; $("app").hidden = false;
  pintarCuenta(r.data.cuenta); msg($("login-msg"), "");
  cargarHistorial();
};

$("btn-logout").onclick = async () => {
  await api("/api/logout", "POST");
  token = null; $("app").hidden = true; $("login").hidden = false; $("json").hidden = true;
};

async function operar(tipo) {
  const txt = $("monto").value.trim();
  const r = await api("/api/" + tipo, "POST", { monto: txt });
  if (r.data.cuenta) pintarCuenta(r.data.cuenta);
  if (r.ok) {
    const t = r.data.transaccion;
    msg($("op-msg"), (tipo === "retiro" ? "Retiro" : "Abono") + " exitoso (" + t.id + "). Nuevo saldo: " + money(t.saldo_despues), "ok");
    $("monto").value = "";
  } else {
    const e = r.data.error || { mensaje: "Error inesperado" };
    const t = r.data.transaccion;
    msg($("op-msg"), e.mensaje + (t ? " (registrada como " + t.id + ")" : ""), t && t.estado === "RECHAZADA" ? "warn" : "err");
  }
  cargarHistorial();
}
$("btn-retiro").onclick = () => operar("retiro");
$("btn-abono").onclick = () => operar("abono");
$("btn-json").onclick = () => { $("json").hidden = !$("json").hidden; };
document.addEventListener("keydown", (e) => {
  if (e.key === "Enter") { if (!$("login").hidden) $("btn-login").click(); else $("btn-retiro").click(); }
});
