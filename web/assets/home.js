(() => {
  const reduceMotion = matchMedia("(prefers-reduced-motion: reduce)").matches;
  const saveData = !!(navigator.connection && navigator.connection.saveData);
  const allowVideo = !reduceMotion && !saveData;

  const revealIO = new IntersectionObserver((entries) => {
    entries.forEach((e) => {
      if (!e.isIntersecting) return;
      e.target.classList.add("in");
      revealIO.unobserve(e.target);
    });
  }, { threshold: 0.12, rootMargin: "0px 0px -6% 0px" });
  document.querySelectorAll(".reveal").forEach((el, i) => {
    el.style.transitionDelay = `${(i % 4) * 70}ms`;
    revealIO.observe(el);
  });

  const hero = document.querySelector("video[data-autoplay]");
  if (hero && !allowVideo) {
    hero.removeAttribute("autoplay");
    hero.pause();
  }

  const lazy = document.querySelectorAll("video[data-lazy]");
  const videoIO = new IntersectionObserver((entries) => {
    entries.forEach(({ target: v, isIntersecting }) => {
      if (isIntersecting) {
        const src = v.querySelector("source[data-src]");
        if (src) {
          src.src = src.dataset.src;
          src.removeAttribute("data-src");
          v.load();
        }
        v.play().catch(() => {});
      } else if (!v.paused) {
        v.pause();
      }
    });
  }, { rootMargin: "200px 0px", threshold: 0.01 });
  if (allowVideo) lazy.forEach((v) => videoIO.observe(v));

  if (hero && allowVideo) {
    new IntersectionObserver(([e]) => {
      if (e.isIntersecting) hero.play().catch(() => {});
      else hero.pause();
    }).observe(hero);
  }

  const film = document.getElementById("film");
  const filmVideo = film.querySelector("video");
  const openFilm = () => {
    const lang = document.documentElement.lang === "en" ? "en" : "zh";
    const src = `/assets/video/film-${lang}.mp4`;
    if (!filmVideo.src.endsWith(src)) {
      filmVideo.poster = `/assets/video/film-${lang}.jpg`;
      filmVideo.src = src;
    }
    if (typeof film.showModal === "function") film.showModal();
    else film.setAttribute("open", "");
    filmVideo.currentTime = 0;
    filmVideo.play().catch(() => {});
  };
  const closeFilm = () => {
    filmVideo.pause();
    if (film.open) film.close ? film.close() : film.removeAttribute("open");
  };
  document.querySelectorAll("[data-film]").forEach((b) => b.addEventListener("click", openFilm));
  document.getElementById("filmClose").addEventListener("click", closeFilm);
  film.addEventListener("click", (e) => { if (e.target === film) closeFilm(); });
  film.addEventListener("close", () => filmVideo.pause());
})();
