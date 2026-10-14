// The admin UI's only script.
//
// It is external rather than inline because the response carries
// `Content-Security-Policy: default-src 'self'`, which rejects an inline script
// outright — a page that works until the headers are turned on is a page that does
// not work. For the same reason no element carries an inline click attribute:
// every handler is attached below with addEventListener.
//
// There is no framework. The page is a table of children and three buttons; a
// build step would be a second thing to install on the machine that builds this.
(function () {
  "use strict";

  // The CSRF token is injected into the page as a meta tag by the template, not
  // read from a cookie: a script that can read the session cookie is a script that
  // is already the admin, and the token's whole value is that a cross-site page
  // cannot read *this*.
  function csrfToken() {
    var meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute("content") : "";
  }

  // Every state change is a POST with the token, including the ones that used to
  // be links. A GET that changes state is a state change an <img> tag can cause.
  function post(path, fields) {
    var form = document.createElement("form");
    form.method = "post";
    form.action = path;
    function add(name, value) {
      var input = document.createElement("input");
      input.type = "hidden";
      input.name = name;
      input.value = value;
      form.appendChild(input);
    }
    add("csrf", csrfToken());
    for (var key in fields) {
      if (Object.prototype.hasOwnProperty.call(fields, key)) {
        add(key, fields[key]);
      }
    }
    document.body.appendChild(form);
    form.submit();
  }

  // A button that does something irreversible says so once. The alternative is a
  // confirm() dialog on every click, which teaches people to click through
  // warnings — and this UI has exactly one irreversible action.
  function needsConfirm(action) {
    return action === "revoke-all";
  }

  function onClick(event) {
    var button = event.target.closest("button[data-action]");
    if (!button) {
      return;
    }
    event.preventDefault();
    var action = button.getAttribute("data-action");
    if (needsConfirm(action) && !window.confirm("Sign out every device?")) {
      return;
    }
    button.disabled = true;
    post("/api/" + action, {
      user: button.getAttribute("data-user") || "",
      minutes: button.getAttribute("data-minutes") || "",
      reason: button.getAttribute("data-reason") || ""
    });
  }

  document.addEventListener("click", onClick);

  // The clock on a lockout is the one number on the page that goes stale while it
  // is open, and "when does this end" is the question the parent is asking. It is
  // recomputed locally rather than polled: the answer is a fixed instant, so a
  // request would only tell us what the server already told us.
  function renderCountdowns() {
    var nodes = document.querySelectorAll("time[data-unix]");
    for (var i = 0; i < nodes.length; i++) {
      var el = nodes[i];
      var at = parseInt(el.getAttribute("data-unix"), 10) * 1000;
      var seconds = Math.round((at - Date.now()) / 1000);
      if (isNaN(at)) {
        continue;
      }
      el.textContent = seconds > 0 ? format(seconds) : "now";
    }
  }

  function format(seconds) {
    var h = Math.floor(seconds / 3600);
    var m = Math.floor((seconds % 3600) / 60);
    var s = seconds % 60;
    if (h > 0) {
      return h + "h " + m + "m";
    }
    if (m > 0) {
      return m + "m " + s + "s";
    }
    return s + "s";
  }

  renderCountdowns();
  window.setInterval(renderCountdowns, 1000);
})();
