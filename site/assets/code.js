// The code pages of riemann.tamnd.com: the search of Lean names, the page of a declaration and the page of a module. The data is in /api/names/, /api/decl/ and /api/module/. The shards use the FNV-1a hash of the name, as site/code.go does.
(function () {
  "use strict";

  var script = document.currentScript;
  var leanBase = script ? script.getAttribute("data-lean") : "";
  var cache = {};

  function fnv(s) {
    var bytes = new TextEncoder().encode(s);
    var h = 0x811c9dc5;
    for (var i = 0; i < bytes.length; i++) {
      h ^= bytes[i];
      h = Math.imul(h, 0x01000193) >>> 0;
    }
    return h >>> 0;
  }
  function hex(n, width) {
    var s = n.toString(16);
    while (s.length < width) s = "0" + s;
    return s;
  }
  function declShard(name) { return hex(fnv(name) % 4096, 3); }
  function moduleShard(name) { return hex(fnv(name) % 256, 2); }
  function lastPart(name) { return name.slice(name.lastIndexOf(".") + 1); }
  function nameKey(name) {
    var last = lastPart(name).toLowerCase();
    var k = "";
    for (var ch of last) {
      if (k.length === 2) break;
      k += /^[a-z0-9]$/.test(ch) ? ch : "_";
    }
    while (k.length < 2) k += "_";
    return k;
  }
  function q(name) {
    return encodeURIComponent(name).replace(/%2E/gi, ".").replace(/%27/g, "'");
  }
  function declURL(name) { return "/code/decl/?n=" + q(name); }
  function moduleURL(name) { return "/code/module/?m=" + q(name); }
  function sourceURL(file, line) { return leanBase + file + (line ? "#L" + line : ""); }

  function load(url) {
    if (!cache[url]) {
      cache[url] = fetch(url).then(function (r) {
        if (r.status === 404) return [];
        if (!r.ok) throw new Error(url + ": " + r.status);
        return r.json();
      });
    }
    return cache[url];
  }

  // el makes an element. A string child is text, so no data from the records is read as HTML.
  function el(tag, attrs, kids) {
    var e = document.createElement(tag);
    for (var k in attrs || {}) {
      if (attrs[k] !== undefined && attrs[k] !== null) e.setAttribute(k, attrs[k]);
    }
    (kids || []).forEach(function (c) {
      if (c === null || c === undefined) return;
      e.appendChild(typeof c === "string" || typeof c === "number" ? document.createTextNode(String(c)) : c);
    });
    return e;
  }
  function code(name, url) {
    var c = el("code", {}, [name]);
    return url ? el("a", { href: url }, [c]) : c;
  }
  function num(n) { return Number(n).toLocaleString("en-US"); }
  function mark(text, cls, title) { return el("span", { class: "mark" + (cls ? " " + cls : ""), title: title }, [text]); }

  // The search of Lean names.
  function search(input, list) {
    var timer = 0;
    var seq = 0;
    function run() {
      var text = input.value.trim();
      list.textContent = "";
      if (lastPart(text).length < 2) return;
      var my = ++seq;
      var want = text.toLowerCase();
      var wantLast = lastPart(want);
      load("/api/names/" + nameKey(text) + ".json").then(function (rows) {
        if (my !== seq) return;
        var hits = rows.filter(function (r) {
          var n = r[0].toLowerCase();
          return lastPart(n).indexOf(wantLast) === 0 && (want.indexOf(".") < 0 || n.indexOf(want) >= 0);
        });
        hits.sort(function (a, b) {
          var ea = lastPart(a[0]).toLowerCase() === wantLast ? 0 : 1;
          var eb = lastPart(b[0]).toLowerCase() === wantLast ? 0 : 1;
          if (ea !== eb) return ea - eb;
          var sa = a[2] >= 0 ? 0 : 1, sb = b[2] >= 0 ? 0 : 1;
          if (sa !== sb) return sa - sb;
          return a[0].length - b[0].length || (a[0] < b[0] ? -1 : 1);
        });
        if (!hits.length) {
          list.appendChild(el("li", { class: "muted" }, ["No Lean name starts with " + lastPart(text) + "."]));
          return;
        }
        hits.slice(0, 40).forEach(function (r) {
          list.appendChild(el("li", {}, [
            el("a", { href: declURL(r[0]) }, [el("code", {}, [r[0]])]), " ",
            el("span", { class: "muted" }, [r[1] + (r[2] >= 0 ? ", depth " + r[2] + " on the spine" : "")])
          ]));
        });
        if (hits.length > 40) list.appendChild(el("li", { class: "muted" }, [num(hits.length) + " names match. Type more of the name to see the others."]));
      }).catch(function () {
        list.appendChild(el("li", { class: "muted" }, ["The Lean names are not on this site yet."]));
      });
    }
    input.addEventListener("input", function () {
      clearTimeout(timer);
      timer = setTimeout(run, 150);
    });
    if (input.value) run();
  }

  function names(title, all, total, more) {
    if (!all || !all.length) return null;
    var s = el("section", {}, [el("h2", {}, [title + " (" + num(total || all.length) + ")"])]);
    var ul = el("ul", { class: "names" });
    all.forEach(function (n) { ul.appendChild(el("li", {}, [code(n, declURL(n))])); });
    s.appendChild(ul);
    if (total && total > all.length) s.appendChild(el("p", { class: "muted" }, [more]));
    return s;
  }

  function showDecl(box, r, many) {
    var head = el("header", { class: "node-head" }, [
      el(many ? "h2" : "h1", { class: "decl-name" }, [r.n]),
      el("p", { class: "decl-marks" }, [
        mark(r.k), " ",
        r.p ? mark("private", "", "Only its own module can use it") : null, r.p ? " " : null,
        r.x ? mark("on the spine", "yes", "The Riemann Hypothesis uses it") : mark("not on the spine", "", "The Riemann Hypothesis does not use it, as far as the syntactic graph shows"), " ",
        r.c ? mark("named result", "open", "A result that the code states with a name from the literature. Spec 04.6 of riemann-agent tells how the site treats it.") : null
      ])
    ]);
    box.appendChild(head);
    var facts = el("table", { class: "checks facts" }, [
      el("tr", {}, [el("th", {}, ["Module"]), el("td", {}, [code(r.m, moduleURL(r.m))])]),
      el("tr", {}, [el("th", {}, ["Source"]), el("td", {}, [el("a", { href: sourceURL(r.f, r.l) }, [r.f + ", line " + r.l])])])
    ]);
    if (r.x) {
      facts.appendChild(el("tr", {}, [el("th", {}, ["Depth"]), el("td", {}, [r.x.d + " steps of use from the top"])]));
      facts.appendChild(el("tr", {}, [el("th", {}, ["Size"]), el("td", {}, [num(r.x.s) + " declarations that only it leads to"])]));
      if (r.x.i) facts.appendChild(el("tr", {}, [el("th", {}, ["Through"]), el("td", {}, [code(r.x.i, declURL(r.x.i)), el("span", { class: "sub" }, [" each chain of uses from the top goes through it"])])]));
      if (r.x.y) facts.appendChild(el("tr", {}, [el("th", {}, ["Part"]), el("td", {}, [r.x.y === "trunk" ? "the trunk" : code(r.x.y, declURL(r.x.y))])]));
    }
    if (r.a) facts.appendChild(el("tr", {}, [el("th", {}, ["Attributes"]), el("td", {}, [el("code", {}, [r.a])])]));
    box.appendChild(facts);
    if (r.d) box.appendChild(el("div", { class: "doc" }, [r.d.replace(/^\/--\s*|\s*-\/$/g, "")]));
    if (r.s) box.appendChild(el("pre", { class: "sig" }, [el("code", {}, [r.n + " " + r.s])]));
    var grid = el("div", { class: "decl-lists" });
    [names("Uses", r.u, r.nu, "The list shows the first names. The source has all."),
      names("Used by", r.b, r.nb, "The list shows the first names."),
      names("Maybe uses", r.w, 0, "")].forEach(function (s) { if (s) grid.appendChild(s); });
    if (r.w && r.w.length) grid.lastChild.appendChild(el("p", { class: "muted" }, ["A name in dot notation can go to more than one declaration. These are the declarations it can go to."]));
    box.appendChild(grid);
  }

  function declPage(box) {
    var status = document.getElementById("decl-status");
    var name = new URLSearchParams(location.search).get("n");
    if (!name) {
      status.textContent = "Give a name. Search the names on the code page.";
      return;
    }
    status.textContent = "Loading " + name + ".";
    load("/api/decl/" + declShard(name) + ".json").then(function (rows) {
      var hits = rows.filter(function (r) { return r.n === name; });
      if (!hits.length) {
        status.textContent = "";
        status.appendChild(document.createTextNode("No declaration has the name " + name + ". "));
        status.appendChild(el("a", { href: "/search/?q=" + q(lastPart(name)) }, ["Search for " + lastPart(name)]));
        return;
      }
      document.title = name + " · riemann";
      box.textContent = "";
      if (hits.length > 1) box.appendChild(el("p", { class: "muted" }, [hits.length + " declarations have this name. One is private to its module."]));
      hits.forEach(function (r) { showDecl(box, r, hits.length > 1); });
      var crumb = document.querySelector(".trail [aria-current]");
      if (crumb) crumb.textContent = lastPart(name);
    }).catch(function (e) { status.textContent = "The page cannot load the declaration: " + e.message; });
  }

  function modulePage(box) {
    var status = document.getElementById("module-status");
    var name = new URLSearchParams(location.search).get("m");
    if (!name) {
      status.textContent = "Give a module name.";
      return;
    }
    load("/api/module/" + moduleShard(name) + ".json").then(function (rows) {
      var m = rows.filter(function (r) { return r.n === name; })[0];
      if (!m) {
        status.textContent = "No module has the name " + name + ".";
        return;
      }
      document.title = name + " · riemann";
      box.textContent = "";
      box.appendChild(el("header", { class: "node-head" }, [el("h1", { class: "decl-name" }, [name])]));
      box.appendChild(el("table", { class: "checks facts" }, [
        el("tr", {}, [el("th", {}, ["Source"]), el("td", {}, [el("a", { href: sourceURL(m.f, 0) }, [m.f]), " (" + num(m.l) + " lines)"])]),
        el("tr", {}, [el("th", {}, ["Depth"]), el("td", {}, [m.d >= 0 ? m.d + " imports from Main" : "Main does not import it"])])
      ]));
      var grid = el("div", { class: "decl-lists" });
      if (m.x.length) {
        var s = el("section", {}, [el("h2", {}, ["Declarations (" + num(m.x.length) + ")"])]);
        var ul = el("ul", { class: "names" });
        m.x.forEach(function (d) {
          ul.appendChild(el("li", {}, [code(d.n, declURL(d.n)), el("span", { class: "sub" }, [" " + d.k + ", line " + d.l + (d.s ? ", on the spine" : "")])]));
        });
        s.appendChild(ul);
        grid.appendChild(s);
      }
      if (m.i.length) {
        var si = el("section", {}, [el("h2", {}, ["Imports (" + num(m.i.length) + ")"])]);
        var ui = el("ul", { class: "names" });
        m.i.forEach(function (n) { ui.appendChild(el("li", {}, [code(n, n.indexOf("Mathlib") === 0 ? null : moduleURL(n))])); });
        si.appendChild(ui);
        grid.appendChild(si);
      }
      box.appendChild(grid);
      var crumb = document.querySelector(".trail [aria-current]");
      if (crumb) crumb.textContent = lastPart(name);
    }).catch(function (e) { status.textContent = "The page cannot load the module: " + e.message; });
  }

  function start() {
    var list = document.getElementById("lean-results");
    var input = document.getElementById("lean-q") || document.getElementById("q");
    if (list && input) search(input, list);
    var d = document.getElementById("decl");
    if (d) declPage(d);
    var m = document.getElementById("module");
    if (m) modulePage(m);
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start);
  else start();
})();
