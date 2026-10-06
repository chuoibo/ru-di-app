/* Progressive enhancement only.
 *
 * With JavaScript off the transfer details are already open and every value is
 * selectable by hand, so the failure mode is "shows more at once", never
 * "cannot pay". Nothing here is required to complete a transfer.
 */
(function () {
  "use strict";

  // A retry from browser history keeps the same domain replay key.
  document.querySelectorAll("form[data-claim-form]").forEach(function (form) {
    if (!window.crypto || !window.crypto.getRandomValues) return;
    var bytes = new Uint8Array(16);
    window.crypto.getRandomValues(bytes);
    bytes[6] = (bytes[6] & 15) | 64;
    bytes[8] = (bytes[8] & 63) | 128;
    var hex = Array.from(bytes, function (byte) { return byte.toString(16).padStart(2, "0"); }).join("");
    var key = document.createElement("input");
    key.type = "hidden";
    key.name = "idempotency_key";
    key.value = [hex.slice(0, 8), hex.slice(8, 12), hex.slice(12, 16), hex.slice(16, 20), hex.slice(20)].join("-");
    form.appendChild(key);
  });

  document.addEventListener("submit", function (event) {
    var form = event.target;
    if (form.dataset.submitting === "1") { event.preventDefault(); return; }
    form.dataset.submitting = "1";
    // Disabled submit buttons are omitted from the browser's form payload.
    var submitter = event.submitter;
    if (submitter && submitter.name) {
      var intent = document.createElement("input");
      intent.type = "hidden";
      intent.name = submitter.name;
      intent.value = submitter.value;
      intent.setAttribute("data-submit-intent", "");
      form.appendChild(intent);
    }
    form.querySelectorAll('button[type="submit"]').forEach(function (button) {
      button.dataset.previousLabel = button.textContent;
      button.disabled = true;
      button.setAttribute("aria-busy", "true");
      button.textContent = "Đang gửi…";
    });
  });

  // Back from a failed request must leave the original form usable.
  window.addEventListener("pageshow", function () {
    document.querySelectorAll('form[data-submitting="1"]').forEach(function (form) {
      delete form.dataset.submitting;
      form.querySelectorAll("[data-submit-intent]").forEach(function (input) { input.remove(); });
      form.querySelectorAll('button[data-previous-label]').forEach(function (button) {
        button.textContent = button.dataset.previousLabel;
        delete button.dataset.previousLabel;
        button.disabled = false;
        button.removeAttribute("aria-busy");
      });
    });
  });

  function feedback(target, message) {
    var card = target.closest(".card");
    var status = card && card.querySelector("[data-copy-feedback]");
    if (status) status.textContent = message;
  }

  function copy(text, target) {
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(
        function () { feedback(target, "Đã chép số tiền này."); },
        function () { feedback(target, "Không chép được. Chọn số tiền rồi chép tay."); }
      );
      return;
    }
    // Older mobile browsers, and any page not served over HTTPS.
    var field = document.createElement("textarea");
    var previousFocus = document.activeElement;
    field.value = text;
    field.setAttribute("readonly", "");
    field.style.position = "fixed";
    field.style.opacity = "0";
    document.body.appendChild(field);
    field.select();
    try {
      feedback(target, document.execCommand("copy") ? "Đã chép số tiền này." : "Không chép được. Chọn số tiền rồi chép tay.");
    } catch (error) {
      feedback(target, "Không chép được. Chọn số tiền rồi chép tay.");
    }
    document.body.removeChild(field);
    if (previousFocus && previousFocus.focus) previousFocus.focus({ preventScroll: true });
  }

  document.addEventListener("click", function (event) {
    var copyTarget = event.target.closest("[data-copy]");
    if (copyTarget) {
      copy(copyTarget.getAttribute("data-copy"), copyTarget);
      return;
    }
    var reveal = event.target.closest("[data-reveal]");
    if (reveal) {
      var card = reveal.closest(".card");
      var panel = card && card.querySelector("[data-transfer]");
      if (panel) {
        panel.classList.add("is-open");
        reveal.setAttribute("aria-expanded", "true");
        reveal.hidden = true;
        var nextControl = panel.querySelector('button:not(:disabled), a[href], input:not([type="hidden"]):not(:disabled)');
        if (!nextControl) panel.setAttribute("tabindex", "-1");
        (nextControl || panel).focus({ preventScroll: true });
        panel.scrollIntoView({ block: "nearest", behavior: "auto" });
      }
    }
  });

  // Collapsing is done in CSS, gated on the .js class the inline script sets
  // before paint. Nothing here hides anything, so a failure in this file can
  // never leave a guest unable to see where to transfer.
})();
