/**
 * toast.js — Sonner-style toast library (Vanilla JS)
 * Usage: see README or demo below
 */

(function(global) {
    "use strict";

    /* ─── Constants ────────────────────────────────────────────────── */
    const CONTAINER_ID = "__toast_container__";
    const DEFAULTS = {
        duration: 4000,
        position: "bottom-center", // top-left | top-center | top-right | bottom-left | bottom-center | bottom-right
        maxVisible: 3,
        gap: 12,
        toastWidth: 420,
    };

    const ICONS = {
        success:
            `<svg viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg"><circle cx="10" cy="10" r="9" stroke="currentColor" stroke-width="1.5"/><path d="M6.5 10.5l2.5 2.5 4.5-5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>`,
        error:
            `<svg viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg"><circle cx="10" cy="10" r="9" stroke="currentColor" stroke-width="1.5"/><path d="M7 7l6 6M13 7l-6 6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>`,
        warning:
            `<svg viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M9.134 3.374c.394-.718 1.338-.718 1.732 0l6.866 12.503C18.124 16.595 17.655 17.5 16.866 17.5H3.134c-.789 0-1.258-.905-.866-1.623L9.134 3.374z" stroke="currentColor" stroke-width="1.5"/><path d="M10 8.5v3.5M10 13.5v.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>`,
        info:
            `<svg viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg"><circle cx="10" cy="10" r="9" stroke="currentColor" stroke-width="1.5"/><path d="M10 9v5M10 7v.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>`,
        loading:
            `<svg viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg" class="__toast_spin__"><circle cx="10" cy="10" r="7.5" stroke="currentColor" stroke-width="1.5" stroke-dasharray="40" stroke-dashoffset="10" stroke-linecap="round"/></svg>`,
    };

    /* ─── CSS Injection ─────────────────────────────────────────────── */
    function injectStyles() {
        if (document.getElementById("__toast_styles__")) return;
        const style = document.createElement("style");
        style.id = "__toast_styles__";
        style.textContent = `
      :root {
        --toast-font: var(--baseFontFamily, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif);
        --toast-bg: var(--surfaceColor, #fff);
        --toast-border: var(--surfaceAlt3Color, rgba(0,0,0,.08));
        --toast-shadow: var(--boxShadow, 0 4px 12px rgba(0,0,0,.08));
        --toast-shadow-hover: var(--boxShadow, 0 8px 24px rgba(0,0,0,.12));
        --toast-text: var(--surfaceTxtColor, #0a0a0a);
        --toast-text-sub: var(--surfaceTxtHintColor, #6b7280);
        --toast-radius: var(--borderRadius, 8px);
        --toast-width: 420px;
        --toast-dismiss-bg: color-mix(in srgb, var(--surfaceAlt1Color, #000), transparent 55%);
        --toast-dismiss-hover: color-mix(in srgb, var(--surfaceAlt2Color, #000), transparent 35%);
        --toast-btn-border: var(--surfaceAlt4Color, rgba(0,0,0,.1));
        --toast-btn-hover: var(--surfaceAlt1Color, rgba(0,0,0,.04));

        /* type colors */
        --toast-success: var(--successColor, #16a34a);
        --toast-success-bg: var(--surfaceSuccessColor, #f0fdf4);
        --toast-success-border: var(--successColor, #bbf7d0);
        --toast-error: var(--dangerColor, #dc2626);
        --toast-error-bg: var(--surfaceDangerColor, #fef2f2);
        --toast-error-border: var(--dangerColor, #fecaca);
        --toast-warning: var(--warningColor, #d97706);
        --toast-warning-bg: var(--surfaceWarningColor, #fffbeb);
        --toast-warning-border: var(--warningColor, #fde68a);
        --toast-info: var(--infoColor, #2563eb);
        --toast-info-bg: var(--surfaceInfoColor, #eff6ff);
        --toast-info-border: var(--infoColor, #bfdbfe);
        --toast-loading: var(--surfaceTxtHintColor, #6b7280);
      }

      /* ── Container ── */
      .__toast_container__ {
        position: fixed;
        z-index: 99999;
        display: flex;
        flex-direction: column;
        gap: 0;
        pointer-events: none;
        font-family: var(--toast-font);
        box-sizing: border-box;
      }

      .__toast_container__[data-position^="bottom"] { flex-direction: column-reverse; }

      .__toast_container__[data-position$="right"]  { right: 16px; }
      .__toast_container__[data-position$="left"]   { left: 16px; }
      .__toast_container__[data-position$="center"] { left: 50%; transform: translateX(-50%); }

      .__toast_container__[data-position^="top"]    { top: 16px; }
      .__toast_container__[data-position^="bottom"] { bottom: 16px; }

      /* ── Toast wrapper (handles stacking) ── */
      .__toast_wrapper__ {
        position: relative;
        transition: all 400ms cubic-bezier(0.32, 0.72, 0, 1);
        pointer-events: none;
      }

      /* ── Toast card ── */
      .__toast__ {
        position: relative;
        display: flex;
        flex-direction: column;
        gap: 0;
        width: var(--toast-width);
        max-width: calc(100vw - 32px);
        background: var(--toast-bg);
        border: 1px solid var(--toast-border);
        border-radius: var(--toast-radius);
        box-shadow: var(--toast-shadow);
        padding: 14px 14px 12px 14px;
        box-sizing: border-box;
        pointer-events: all;
        overflow: hidden;
        transform-origin: center bottom;
        transition:
          transform 400ms cubic-bezier(0.32, 0.72, 0, 1),
          opacity 300ms ease,
          box-shadow 200ms ease;
        cursor: default;
        will-change: transform, opacity;
      }

      .__toast__:hover {
        box-shadow: var(--toast-shadow-hover);
      }

      /* ── Variants ── */
      .__toast__[data-type="success"] { border-color: var(--toast-success-border); background: var(--toast-success-bg); }
      .__toast__[data-type="error"]   { border-color: var(--toast-error-border);   background: var(--toast-error-bg); }
      .__toast__[data-type="warning"] { border-color: var(--toast-warning-border); background: var(--toast-warning-bg); }
      .__toast__[data-type="info"]    { border-color: var(--toast-info-border);    background: var(--toast-info-bg); }

      /* ── Header row ── */
      .__toast_header__ {
        display: flex;
        align-items: flex-start;
        gap: 10px;
      }

      /* ── Icon ── */
      .__toast_icon__ {
        flex-shrink: 0;
        width: 18px;
        height: 18px;
        margin-top: 1px;
        display: flex;
        align-items: center;
        justify-content: center;
      }
      .__toast_icon__ svg { width: 18px; height: 18px; }

      .__toast__[data-type="success"] .__toast_icon__ { color: var(--toast-success); }
      .__toast__[data-type="error"]   .__toast_icon__ { color: var(--toast-error); }
      .__toast__[data-type="warning"] .__toast_icon__ { color: var(--toast-warning); }
      .__toast__[data-type="info"]    .__toast_icon__ { color: var(--toast-info); }
      .__toast__[data-type="loading"] .__toast_icon__ { color: var(--toast-loading); }

      /* ── Content ── */
      .__toast_content__ {
        flex: 1;
        min-width: 0;
        display: flex;
        flex-direction: column;
        gap: 3px;
      }

      .__toast_title__ {
        font-size: 13.5px;
        font-weight: 600;
        color: var(--toast-text);
        line-height: 1.4;
        letter-spacing: 0;
      }

      .__toast_description__ {
        font-size: 12.5px;
        font-weight: 400;
        color: var(--toast-text-sub);
        line-height: 1.5;
      }

      /* ── Dismiss button ── */
      .__toast_dismiss__ {
        flex-shrink: 0;
        width: 24px;
        height: 24px;
        display: flex;
        align-items: center;
        justify-content: center;
        border: none;
        border-radius: 6px;
        background: var(--toast-dismiss-bg);
        color: var(--toast-text-sub);
        cursor: pointer;
        padding: 0;
        margin-top: -2px;
        transition: background 150ms ease, color 150ms ease, transform 150ms ease;
        font-family: inherit;
        line-height: 1;
      }
      .__toast_dismiss__:hover {
        background: var(--toast-dismiss-hover);
        color: var(--toast-text);
        transform: scale(1.05);
      }
      .__toast_dismiss__:active { transform: scale(0.95); }
      .__toast_dismiss__ svg { width: 13px; height: 13px; }

      /* ── Action buttons ── */
      .__toast_actions__ {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-top: 10px;
        padding-top: 10px;
        border-top: 1px solid var(--toast-border);
      }

      .__toast_btn__ {
        flex: 1;
        height: 30px;
        font-size: 12px;
        font-weight: 500;
        font-family: var(--toast-font);
        letter-spacing: 0;
        border-radius: 7px;
        cursor: pointer;
        border: 1px solid var(--toast-btn-border);
        background: transparent;
        color: var(--toast-text);
        transition: background 140ms ease, border-color 140ms ease, transform 120ms ease;
        padding: 0 12px;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }
      .__toast_btn__:hover {
        background: var(--toast-btn-hover);
        border-color: transparent;
      }
      .__toast_btn__:active { transform: scale(0.98); }

      .__toast_btn__.__toast_btn_primary__ {
        background: var(--toast-text);
        color: var(--toast-bg);
        border-color: transparent;
      }
      .__toast_btn__.__toast_btn_primary__:hover {
        opacity: 0.88;
      }

      /* ── Progress bar ── */
      .__toast_progress__ {
        position: absolute;
        bottom: 0;
        left: 0;
        height: 2.5px;
        background: currentColor;
        opacity: 0.2;
        border-radius: 0 0 0 var(--toast-radius);
        transform-origin: left center;
        transition: none;
      }
      .__toast__[data-type="success"] .__toast_progress__ { color: var(--toast-success); opacity: 0.4; }
      .__toast__[data-type="error"]   .__toast_progress__ { color: var(--toast-error);   opacity: 0.4; }
      .__toast__[data-type="warning"] .__toast_progress__ { color: var(--toast-warning); opacity: 0.4; }
      .__toast__[data-type="info"]    .__toast_progress__ { color: var(--toast-info);    opacity: 0.4; }

      /* ── Enter / exit animations ── */
      .__toast_enter_top__ {
        animation: __toastSlideInTop__ 380ms cubic-bezier(0.32, 0.72, 0, 1) forwards;
      }
      .__toast_enter_bottom__ {
        animation: __toastSlideInBottom__ 380ms cubic-bezier(0.32, 0.72, 0, 1) forwards;
      }
      .__toast_exit__ {
        animation: __toastFadeOut__ 280ms cubic-bezier(0.4, 0, 1, 1) forwards !important;
      }

      @keyframes __toastSlideInBottom__ {
        from { opacity: 0; transform: translateY(100%) scale(0.92); }
        to   { opacity: 1; transform: translateY(0) scale(1); }
      }
      @keyframes __toastSlideInTop__ {
        from { opacity: 0; transform: translateY(-100%) scale(0.92); }
        to   { opacity: 1; transform: translateY(0) scale(1); }
      }
      @keyframes __toastFadeOut__ {
        from { opacity: 1; transform: scale(1); }
        to   { opacity: 0; transform: scale(0.88); }
      }

      /* ── Spinner ── */
      .__toast_spin__ {
        animation: __toastSpin__ 800ms linear infinite;
      }
      @keyframes __toastSpin__ {
        to { transform: rotate(360deg); }
      }

      /* ── Hover pause ── */
      .__toast__:hover .__toast_progress__ {
        animation-play-state: paused;
      }
    `;
        document.head.appendChild(style);
    }

    /* ─── Container ────────────────────────────────────────────────── */
    function getContainer(position) {
        let container = document.getElementById(CONTAINER_ID);
        if (!container) {
            container = document.createElement("div");
            container.id = CONTAINER_ID;
            container.className = "__toast_container__";
            document.body.appendChild(container);
        }
        container.setAttribute("data-position", position || DEFAULTS.position);
        return container;
    }

    /* ─── Toast state ───────────────────────────────────────────────── */
    let toastCount = 0;
    const activeToasts = new Map(); // id -> { el, timer, progressAnim }
    const keyedToasts = new Map(); // key -> id

    function isNode(value) {
        return typeof Node !== "undefined" && value instanceof Node;
    }

    function getToastRef(id, options) {
        return options.key || options.title || options.message || id;
    }

    function normalizeToastContent(options) {
        if (typeof options === "string" || isNode(options)) {
            options = { title: options };
        }

        return Object.assign({}, DEFAULTS, options || {});
    }

    function setContent(el, content) {
        if (isNode(content)) {
            el.appendChild(content);
        } else {
            el.textContent = content || "";
        }
    }

    function resolveToastId(toastRef) {
        if (activeToasts.has(toastRef)) {
            return toastRef;
        }

        return keyedToasts.get(toastRef);
    }

    /* ─── Core render ───────────────────────────────────────────────── */
    function render(options) {
        injectStyles();

        const id = ++toastCount;
        const opts = normalizeToastContent(options);
        const position = opts.position || DEFAULTS.position;
        const isTop = position.startsWith("top");
        const container = getContainer(position);
        const toastRef = getToastRef(id, opts);

        if (keyedToasts.has(toastRef)) {
            dismiss(toastRef, false);
        }

        /* Wrapper (for spacing) */
        const wrapper = document.createElement("div");
        wrapper.className = "__toast_wrapper__";
        wrapper.style.marginBottom = isTop ? "0" : DEFAULTS.gap + "px";
        wrapper.style.marginTop = isTop ? DEFAULTS.gap + "px" : "0";

        /* Card */
        const toast = document.createElement("div");
        toast.className = `__toast__ ${isTop ? "__toast_enter_top__" : "__toast_enter_bottom__"}`;
        toast.setAttribute("data-id", id);
        if (opts.type) toast.setAttribute("data-type", opts.type);
        if (opts.toastWidth) {
            toast.style.setProperty(
                "--toast-width",
                typeof opts.toastWidth === "number" ? `${opts.toastWidth}px` : opts.toastWidth,
            );
        }

        /* ── Header ── */
        const header = document.createElement("div");
        header.className = "__toast_header__";

        /* Icon */
        if (opts.icon !== false) {
            const iconSrc = opts.icon || (opts.type && ICONS[opts.type]);
            if (iconSrc) {
                const iconEl = document.createElement("div");
                iconEl.className = "__toast_icon__";
                iconEl.innerHTML = iconSrc;
                header.appendChild(iconEl);
            }
        }

        /* Text */
        const content = document.createElement("div");
        content.className = "__toast_content__";

        const title = document.createElement("div");
        title.className = "__toast_title__";
        setContent(title, opts.title || opts.message || "");
        content.appendChild(title);

        if (opts.description) {
            const desc = document.createElement("div");
            desc.className = "__toast_description__";
            setContent(desc, opts.description);
            content.appendChild(desc);
        }

        header.appendChild(content);

        /* Dismiss button */
        const dismissBtn = document.createElement("button");
        dismissBtn.className = "__toast_dismiss__";
        dismissBtn.setAttribute("aria-label", "Dismiss");
        dismissBtn.innerHTML =
            `<svg viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M2 2l10 10M12 2L2 12" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>`;
        dismissBtn.addEventListener("click", () => dismiss(id));
        header.appendChild(dismissBtn);

        toast.appendChild(header);

        /* ── Action buttons ── */
        if (opts.actions && opts.actions.length) {
            const actionsEl = document.createElement("div");
            actionsEl.className = "__toast_actions__";

            const btns = opts.actions.slice(0, 2); // max 2
            btns.forEach((action, i) => {
                const btn = document.createElement("button");
                btn.className = `__toast_btn__ ${i === 0 ? "__toast_btn_primary__" : ""}`;
                btn.textContent = action.label;
                btn.addEventListener("click", () => {
                    if (typeof action.onClick === "function") action.onClick();
                    if (action.dismissOnClick !== false) dismiss(id);
                });
                actionsEl.appendChild(btn);
            });

            toast.appendChild(actionsEl);
        }

        /* ── Progress bar ── */
        let progressAnim = null;
        const duration = opts.duration !== undefined ? opts.duration : DEFAULTS.duration;

        if (duration > 0 && opts.type !== "loading") {
            const progress = document.createElement("div");
            progress.className = "__toast_progress__";
            progress.style.width = "100%";
            toast.appendChild(progress);

            // CSS animation for smooth progress
            progress.style.animation = `none`;
            // Force reflow
            progress.getBoundingClientRect();
            progress.style.transition = `transform ${duration}ms linear`;
            progress.style.transform = `scaleX(1)`;
            setTimeout(() => {
                progress.style.transform = `scaleX(0)`;
                progress.style.transformOrigin = "left center";
            }, 30);

            progressAnim = progress;
        }

        wrapper.appendChild(toast);

        /* Insert at correct position */
        if (isTop) {
            container.appendChild(wrapper);
        } else {
            container.insertBefore(wrapper, container.firstChild);
        }

        /* Auto-dismiss timer */
        let timer = null;
        if (duration > 0 && opts.type !== "loading") {
            timer = setTimeout(() => dismiss(id), duration);
        }

        /* Pause on hover */
        let elapsed = 0;
        let startTime = Date.now();

        toast.addEventListener("mouseenter", () => {
            if (!timer) return;
            clearTimeout(timer);
            elapsed += Date.now() - startTime;
            if (progressAnim) progressAnim.style.animationPlayState = "paused";
        });

        toast.addEventListener("mouseleave", () => {
            if (duration <= 0 || opts.type === "loading") return;
            const remaining = duration - elapsed;
            if (remaining <= 0) {
                dismiss(id);
                return;
            }
            startTime = Date.now();
            timer = setTimeout(() => dismiss(id), remaining);
        });

        activeToasts.set(id, { el: wrapper, toast, timer, progressAnim, opts, toastRef });
        keyedToasts.set(toastRef, id);

        return id;
    }

    /* ─── Dismiss ───────────────────────────────────────────────────── */
    function dismiss(toastRef, animate = true) {
        const id = resolveToastId(toastRef);
        const entry = activeToasts.get(id);
        if (!entry) return;

        clearTimeout(entry.timer);
        keyedToasts.delete(entry.toastRef);

        if (!animate) {
            if (entry.el && entry.el.parentNode) {
                entry.el.parentNode.removeChild(entry.el);
            }
            activeToasts.delete(id);
            return;
        }

        entry.toast.classList.remove("__toast_enter_top__", "__toast_enter_bottom__");
        entry.toast.classList.add("__toast_exit__");

        setTimeout(() => {
            if (entry.el && entry.el.parentNode) {
                entry.el.parentNode.removeChild(entry.el);
            }
            activeToasts.delete(id);
        }, 300);
    }

    /* ─── Update (for promise/loading) ────────────────────────────── */
    function update(id, options) {
        const entry = activeToasts.get(id);
        if (!entry) return;

        const { toast, opts: prevOpts } = entry;
        const newOpts = Object.assign({}, prevOpts, options);

        // Update type
        if (options.type !== undefined) {
            toast.setAttribute("data-type", options.type || "");
        }

        // Update title
        if (options.title !== undefined || options.message !== undefined) {
            const titleEl = toast.querySelector(".__toast_title__");
            if (titleEl) {
                titleEl.textContent = "";
                setContent(titleEl, options.title || options.message || "");
            }
        }

        // Update description
        if (options.description !== undefined) {
            let descEl = toast.querySelector(".__toast_description__");
            if (!descEl) {
                descEl = document.createElement("div");
                descEl.className = "__toast_description__";
                const content = toast.querySelector(".__toast_content__");
                if (content) content.appendChild(descEl);
            }
            descEl.textContent = "";
            setContent(descEl, options.description);
        }

        // Update icon
        const iconEl = toast.querySelector(".__toast_icon__");
        const newIconSrc = options.icon || (options.type && ICONS[options.type]);
        if (iconEl && newIconSrc) {
            iconEl.innerHTML = newIconSrc;
        } else if (!iconEl && newIconSrc) {
            const newIcon = document.createElement("div");
            newIcon.className = "__toast_icon__";
            newIcon.innerHTML = newIconSrc;
            const header = toast.querySelector(".__toast_header__");
            if (header) header.insertBefore(newIcon, header.firstChild);
        }

        // Reset timer
        clearTimeout(entry.timer);
        const duration = options.duration !== undefined ? options.duration : DEFAULTS.duration;
        if (duration > 0 && newOpts.type !== "loading") {
            entry.timer = setTimeout(() => dismiss(id), duration);
        }

        entry.opts = newOpts;
        const nextToastRef = getToastRef(id, newOpts);
        if (nextToastRef !== entry.toastRef) {
            keyedToasts.delete(entry.toastRef);
            keyedToasts.set(nextToastRef, id);
            entry.toastRef = nextToastRef;
        }
    }

    /* ─── Promise helper ────────────────────────────────────────────── */
    function promise(promiseFn, options) {
        const id = render({
            type: "loading",
            title: options.loading || "Loading…",
            duration: 0,
            position: options.position,
        });

        Promise.resolve(typeof promiseFn === "function" ? promiseFn() : promiseFn)
            .then((data) => {
                const successTitle = typeof options.success === "function"
                    ? options.success(data)
                    : options.success || "Done!";
                update(id, {
                    type: "success",
                    title: successTitle,
                    duration: options.duration,
                });
            })
            .catch((err) => {
                const errorTitle = typeof options.error === "function"
                    ? options.error(err)
                    : options.error || "Something went wrong";
                update(id, {
                    type: "error",
                    title: errorTitle,
                    duration: options.duration,
                });
            });

        return id;
    }

    /* ─── Dismiss all ───────────────────────────────────────────────── */
    function dismissAll(animate = true) {
        removeAll(animate);
    }

    function removeAll(animate = true) {
        Array.from(activeToasts.keys()).forEach((id) => dismiss(id, animate));
    }

    /* ─── Public API ────────────────────────────────────────────────── */
    const toast = function(options) {
        return render(options);
    };

    toast.success = (title, opts) => render(Object.assign({ type: "success", title }, opts));

    toast.error = (title, opts) => render(Object.assign({ type: "error", title }, opts));

    toast.warning = (title, opts) => render(Object.assign({ type: "warning", title }, opts));

    toast.info = (title, opts) => render(Object.assign({ type: "info", title }, opts));

    toast.loading = (title, opts) => render(Object.assign({ type: "loading", title, duration: 0 }, opts));

    toast.promise = promise;
    toast.dismiss = dismiss;
    toast.dismissAll = dismissAll;
    toast.remove = dismiss;
    toast.removeAll = removeAll;
    toast.update = update;

    function infoToast(textOrElem, options = {}) {
        return render(Object.assign({ type: "info", title: textOrElem, duration: 3000 }, options));
    }

    function successToast(textOrElem, options = {}) {
        return render(Object.assign({ type: "success", title: textOrElem, duration: 3000 }, options));
    }

    function errorToast(textOrElem, options = {}) {
        return render(Object.assign({ type: "error", title: textOrElem, duration: 3500 }, options));
    }

    /* ─── Export ────────────────────────────────────────────────────── */
    if (typeof module !== "undefined" && module.exports) {
        module.exports = toast;
    } else if (typeof define === "function" && define.amd) {
        define(function() {
            return toast;
        });
    } else {
        global.toast = toast;
    }

    global.app = global.app || {};
    global.app.toasts = global.app.toasts || {};
    global.app.toasts.info = infoToast;
    global.app.toasts.error = errorToast;
    global.app.toasts.success = successToast;
    global.app.toasts.warning = toast.warning;
    global.app.toasts.loading = toast.loading;
    global.app.toasts.promise = toast.promise;
    global.app.toasts.update = toast.update;
    global.app.toasts.remove = dismiss;
    global.app.toasts.removeAll = removeAll;
    global.app.toasts.toast = toast;
})(typeof globalThis !== "undefined" ? globalThis : typeof window !== "undefined" ? window : this);
