(function () {
  'use strict';

  var STATUS = ['angefragt', 'bestätigt', 'in Arbeit', 'fertig', 'abgeholt'];
  var SCLS = ['angefragt', 'bestaetigt', 'inArbeit', 'fertig', 'abgeholt'];
  var HOURLY_RATE_CENTS = 7800;

  var nfDec = new Intl.NumberFormat('de-DE', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  var nfInt = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 0 });
  var nfCur = new Intl.NumberFormat('de-DE', { style: 'currency', currency: 'EUR' });

  function cents(v) { return nfCur.format(v / 100); }
  function eur(v) { return nfCur.format(v); }
  function dec(v) { return nfDec.format(v); }
  function int(v) { return nfInt.format(v); }
  function hrs(v) { return nfDec.format(v) + ' h'; }
  function qty(v) { return nfInt.format(v) + ' Stück'; }
  function km(v) { return nfInt.format(v) + ' km'; }
  function pct(v) { return nfInt.format(v) + '\u00A0%'; }

  function badge(status) {
    var i = STATUS.indexOf(status);
    var cls = SCLS[Math.max(0, i)];
    return '<span class="status-badge status-' + cls + '"><span class="dot"></span>' + status + '</span>';
  }

  function invoiceFrom(positions) {
    var netto = positions.reduce(function (s, p) { return s + p.sum; }, 0);
    var mwst = Math.round(netto * 0.19);
    return { netto: netto, mwst: mwst, brutto: netto + mwst };
  }

  function setLoading(btn, on) {
    if (!btn) return;
    btn.classList.toggle('is-loading', !!on);
    btn.disabled = !!on;
  }

  function esc(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }

  function setFieldError(input, msg) {
    var id = input.getAttribute('aria-describedby');
    var errEl = id ? document.getElementById(id) : null;
    if (!errEl) errEl = document.getElementById('err-' + input.id);
    if (errEl) {
      errEl.textContent = msg || '';
      errEl.hidden = !msg;
    }
    input.classList.toggle('is-invalid', !!msg);
  }

  function checkField(input, msg) {
    var bad = !String(input.value).trim();
    setFieldError(input, bad ? msg : '');
    return !bad;
  }

  function openModal(backdrop, trigger, firstField) {
    trigger = trigger || document.activeElement;
    backdrop.hidden = false;
    if (firstField) { try { firstField.focus(); } catch (e) {} }
    function onKey(e) {
      if (e.key === 'Escape') {
        close();
      } else if (e.key === 'Tab') {
        var focusable = backdrop.querySelectorAll('button, input, select, textarea, [tabindex]:not([tabindex="-1"])');
        if (!focusable.length) return;
        var first = focusable[0];
        var last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
        else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
      }
    }
    function close() {
      document.removeEventListener('keydown', onKey, true);
      backdrop.removeEventListener('click', onBg);
      if (trigger && trigger.focus) { try { trigger.focus(); } catch (e) {} }
    }
    function onBg(e) {
      if (e.target === backdrop) close();
    }
    document.addEventListener('keydown', onKey, true);
    backdrop.addEventListener('click', onBg);
    setTimeout(function () {
      if (!backdrop.hidden && firstField && document.activeElement !== firstField) try { firstField.focus(); } catch (e) {}
    }, 30);
  }

  function closeModal(backdrop) {
    backdrop.hidden = true;
  }

  function timelineHTML(entries) {
    var html = '<ul class="timeline">';
    for (var i = 0; i < entries.length; i++) {
      var e = entries[i];
      var cls = 'timeline-item tl-' + SCLS[i] + (e.reached ? ' is-reached' : '');
      html += '<li class="' + cls + '">';
      html += '<div class="timeline-row">' + badge(e.status);
      if (e.reached && e.time) {
        html += '<span class="timeline-time">' + e.time + '</span>';
        if (e.actor) html += '<span class="timeline-actor">' + e.actor + '</span>';
      }
      html += '</div></li>';
    }
    html += '</ul>';
    return html;
  }

  function makeTimeline(times, currentStatus) {
    var cur = STATUS.indexOf(currentStatus);
    return times.map(function (t, i) {
      return {
        status: STATUS[i],
        reached: i <= cur,
        time: i <= cur ? t : null,
        actor: i <= cur ? (i === 0 ? 'von System' : 'von Anna Meier') : null
      };
    });
  }

  function initHeader() {
    var toggle = document.querySelector('.nav-toggle');
    var panel = document.getElementById('mobile-panel');
    if (!toggle || !panel) return;
    toggle.addEventListener('click', function () {
      var open = panel.classList.toggle('open');
      toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
      toggle.setAttribute('aria-label', open ? 'Menü schließen' : 'Menü öffnen');
    });
    panel.querySelectorAll('a, button').forEach(function (el) {
      el.addEventListener('click', function () {
        panel.classList.remove('open');
        toggle.setAttribute('aria-expanded', 'false');
      });
    });
  }

  window.wk = {
    STATUS: STATUS,
    SCLS: SCLS,
    HOURLY_RATE_CENTS: HOURLY_RATE_CENTS,
    fmt: { cents: cents, eur: eur, dec: dec, int: int, hrs: hrs, qty: qty, km: km, pct: pct },
    badge: badge,
    invoiceFrom: invoiceFrom,
    setLoading: setLoading,
    esc: esc,
    setFieldError: setFieldError,
    checkField: checkField,
    openModal: openModal,
    closeModal: closeModal,
    timelineHTML: timelineHTML,
    makeTimeline: makeTimeline,
    initHeader: initHeader
  };

  document.addEventListener('DOMContentLoaded', initHeader);

  window.wkOrders = [
    { num: 'AUF-2026-000123', plate: 'B-AB 1234', vehicle: 'VW Golf 8', status: 'bestätigt', date: '20.10.2026, 10:00 Uhr', brutto: null },
    { num: 'AUF-2026-000124', plate: 'B-CD 5678', vehicle: 'BMW 3er (G20)', status: 'in Arbeit', date: '12.10.2026, 08:00 Uhr', brutto: null },
    { num: 'AUF-2026-000125', plate: 'M-EF 9012', vehicle: 'Opel Corsa F', status: 'fertig', date: '08.10.2026, 09:00 Uhr', brutto: 33844 },
    { num: 'AUF-2026-000126', plate: 'HH-GI 3456', vehicle: 'Mercedes A-Klasse (W177)', status: 'abgeholt', date: '07.10.2026, 13:30 Uhr', brutto: 51210 },
    { num: 'AUF-2026-000127', plate: 'B-JK 7890', vehicle: 'Audi A4 Avant', status: 'angefragt', date: '15.10.2026, 09:00 Uhr', brutto: null },
    { num: 'AUF-2026-000122', plate: 'B-LM 2345', vehicle: 'Škoda Octavia', status: 'abgeholt', date: '02.10.2026, 11:00 Uhr', brutto: 17890 },
    { num: 'AUF-2026-000121', plate: 'M-KF 5634', vehicle: 'VW Polo', status: 'fertig', date: '30.09.2026, 15:00 Uhr', brutto: 9642 }
  ];
})();