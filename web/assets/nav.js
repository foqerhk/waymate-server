(() => {
  const STORES = ["appstore", "googleplay", "yyb", "huawei"];
  const ORDER = {
    zh: ["appstore", "huawei", "yyb", "googleplay"],
    en: ["appstore", "googleplay", "huawei", "yyb"],
  };
  const ICON = {
    appstore: "/assets/stores/apple.svg",
    googleplay: "/assets/stores/googleplay.svg",
    yyb: "/assets/stores/yyb.svg",
    huawei: "/assets/stores/huawei.svg",
  };
  let links = {};

  const lang = () => (currentLang() === "en" ? "en" : "zh");
  const esc = (s) => String(s ?? "").replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));

  function badge(key, pack) {
    const url = links[key];
    const inner = `<img src="${ICON[key]}" alt="" width="28" height="28" />
      <span class="sb-txt"><small>${esc(pack[`dl_${key}_s`])}</small><b>${esc(pack[`dl_${key}_t`])}</b></span>
      ${url ? "" : `<em class="sb-soon">${esc(pack.dl_soon)}</em>`}`;
    return url
      ? `<a class="store-badge" data-store="${key}" href="${esc(url)}" target="_blank" rel="noopener">${inner}</a>`
      : `<span class="store-badge is-off" data-store="${key}" aria-disabled="true" title="${esc(pack.dl_soon)}">${inner}</span>`;
  }

  function renderStores() {
    const pack = I18N[lang()] || I18N.zh;
    const html = ORDER[lang()].map((k) => badge(k, pack)).join("");
    document.querySelectorAll("[data-stores]").forEach((el) => { el.innerHTML = html; });
  }

  async function loadStores() {
    try {
      const res = await fetch("/v1/public/download", { cache: "no-store" });
      if (!res.ok) throw new Error(String(res.status));
      const data = await res.json();
      const s = (data && data.stores) || {};
      links = {};
      STORES.forEach((k) => { if (typeof s[k] === "string" && /^(https?|market):\/\//.test(s[k])) links[k] = s[k]; });
    } catch {
      links = {};
    }
    renderStores();
  }

  document.addEventListener("langchange", renderStores);

  document.addEventListener("DOMContentLoaded", () => {
    const nav = document.getElementById("nav");
    const btn = document.getElementById("menuBtn");
    const sheet = document.getElementById("mMenu");

    const onScroll = () => nav.classList.toggle("solid", scrollY > 24 || nav.classList.contains("open"));
    addEventListener("scroll", onScroll, { passive: true });
    onScroll();

    const setOpen = (open) => {
      nav.classList.toggle("open", open);
      document.documentElement.classList.toggle("menu-open", open);
      if (btn) btn.setAttribute("aria-expanded", String(open));
      if (sheet) sheet.inert = !open;
      onScroll();
    };
    if (btn && sheet) {
      sheet.inert = true;
      btn.addEventListener("click", () => setOpen(!nav.classList.contains("open")));
      sheet.addEventListener("click", (e) => { if (e.target.closest("a")) setOpen(false); });
      addEventListener("keydown", (e) => { if (e.key === "Escape") setOpen(false); });
      matchMedia("(min-width: 961px)").addEventListener("change", (m) => { if (m.matches) setOpen(false); });
    }

    renderStores();
    loadStores();
  });
})();
