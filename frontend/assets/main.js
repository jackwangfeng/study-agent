// StudyAgent frontend — Alpine component + helpers.
//
// API base is read from a global injected by index.html (or window.location)
// so the same bundle works in dev (localhost:8000) and prod (current host).
// Keep the rendering pipeline simple: Markdown → HTML → KaTeX auto-render
// (which picks up \( \) and \[ \] math AND \ce{...} thanks to mhchem).

const API_BASE = (() => {
  if (window.STUDY_AGENT_API_BASE) return window.STUDY_AGENT_API_BASE;
  // Dev: backend assumed on :8000 of whatever host serves the frontend.
  // Works for localhost AND LAN testing (iPad → Mac's 192.168.x.x:8000).
  // In prod we'll set STUDY_AGENT_API_BASE explicitly via index.html so
  // frontend on https://study.recompdaily.com calls https://api.../v1.
  if (location.port === "5173" || location.port === "8888") {
    return `http://${location.hostname}:8000/v1`;
  }
  return `${location.origin}/v1`;
})();

document.addEventListener("alpine:init", () => {
  Alpine.data("chemApp", () => ({
    text: "",
    pendingImage: null, // base64 data URL of the photo waiting to be sent
    messages: [], // {role: "user"|"assistant", text, html, image?}
    loading: false,
    // Empty-state quick prompts — give a 高二 student something concrete
    // to tap so the first interaction is "this thing actually answered me",
    // not "I have to think of a question myself".
    quickPrompts: [
      "什么是氧化还原反应？",
      "Cu 被浓硝酸氧化方程式",
      "乙醇怎么变乙酸？",
    ],

    init() {
      // Re-render KaTeX whenever messages change. We listen on a custom
      // hook fired from `pushMessage` instead of a MutationObserver because
      // we know exactly when new content lands and don't want the cost of
      // observing the whole DOM.
      this.$watch("messages", () => this.$nextTick(() => this.renderMath()));

      // PWA: register service worker so iPhone "Add to Home Screen" gets a
      // proper offline-fallback experience. Errors are swallowed — the page
      // works fine without SW, registration only fails on http:// dev.
      if ("serviceWorker" in navigator) {
        navigator.serviceWorker.register("/sw.js").catch(() => {});
      }
    },

    reset() {
      this.messages = [];
      this.text = "";
      this.pendingImage = null;
    },

    async onPick(e) {
      const f = e.target.files?.[0];
      if (!f) return;
      // Read as data URL — keeps the image inline so a single POST sends both
      // photo + text. Good enough up to ~3MB; if iPhone produces 12MP HEIC
      // pictures, the browser auto-converts to JPEG via accept="image/*".
      this.pendingImage = await fileToDataURL(f);
      e.target.value = "";
    },

    async send() {
      const text = this.text.trim();
      if (!text && !this.pendingImage) return;
      this.pushMessage({
        role: "user",
        text: text,
        html: text ? markdownToHTML(text) : "<em class='text-neutral-500'>（拍了张题）</em>",
        image: this.pendingImage,
      });
      const payload = {
        image_data: this.pendingImage || undefined,
        question: text || undefined,
      };
      this.text = "";
      this.pendingImage = null;
      this.loading = true;

      try {
        const resp = await fetch(`${API_BASE}/chem/solve`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });
        if (!resp.ok) {
          throw new Error(`后端 ${resp.status}`);
        }
        const data = await resp.json();
        this.pushMessage({
          role: "assistant",
          text: data.answer || "(空回复)",
          html: markdownToHTML(data.answer || "(空回复)"),
        });
      } catch (err) {
        this.pushMessage({
          role: "assistant",
          text: `请求失败：${err.message}`,
          html: `<span class='text-red-400'>请求失败：${err.message}</span>`,
        });
      } finally {
        this.loading = false;
      }
    },

    pushMessage(m) {
      this.messages = [...this.messages, m];
      this.$nextTick(() => {
        window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });
      });
    },

    renderMath() {
      // window.renderMathInElement is provided by KaTeX auto-render. Configure
      // delimiters so both LaTeX-style (\(...\), $$...$$) and inline mhchem
      // (\ce{...}) all work. trust:true is required for mhchem to not be
      // sandboxed away.
      if (!window.renderMathInElement) return;
      try {
        window.renderMathInElement(document.body, {
          delimiters: [
            { left: "$$", right: "$$", display: true },
            { left: "\\[", right: "\\]", display: true },
            { left: "\\(", right: "\\)", display: false },
            { left: "$", right: "$", display: false },
          ],
          throwOnError: false,
          trust: true,
          strict: "ignore",
        });
      } catch (_) {
        // KaTeX errors per-node are non-fatal; rendering falls back to text.
      }
    },
  }));
});

function fileToDataURL(file) {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(r.result);
    r.onerror = () => reject(r.error);
    r.readAsDataURL(file);
  });
}

function markdownToHTML(md) {
  if (!window.marked) return escapeHTML(md);
  return window.marked.parse(md, { gfm: true, breaks: true });
}

function escapeHTML(s) {
  return s.replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;",
    '"': "&quot;", "'": "&#39;",
  }[c]));
}
