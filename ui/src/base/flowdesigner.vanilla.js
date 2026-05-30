/**
 * FlowDesigner.js — A Vanilla JS flow/node graph library
 * Inspired by React Flow, built with zero dependencies
 * @version 1.0.0
 */

const FlowDesigner = (function() {
    "use strict";

    // ─── Utils ────────────────────────────────────────────────────────────────

    function uid() {
        return Math.random().toString(36).slice(2, 10);
    }

    function clamp(v, min, max) {
        return Math.max(min, Math.min(max, v));
    }

    function deepClone(obj) {
        return JSON.parse(JSON.stringify(obj));
    }

    function emit(target, name, detail) {
        target.dispatchEvent(new CustomEvent(name, { detail, bubbles: true }));
    }

    function escapeHtml(value) {
        return String(value == null ? "" : value)
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;")
            .replace(/'/g, "&#039;");
    }

    // ─── SVG helpers ──────────────────────────────────────────────────────────

    function svgEl(tag, attrs = {}) {
        const el = document.createElementNS("http://www.w3.org/2000/svg", tag);
        for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
        return el;
    }

    function bezierPath(x1, y1, x2, y2) {
        const dx = Math.abs(x2 - x1) * 0.6;
        return `M${x1},${y1} C${x1 + dx},${y1} ${x2 - dx},${y2} ${x2},${y2}`;
    }

    // ─── Constants ────────────────────────────────────────────────────────────

    const HANDLE_RADIUS = 7;
    const NODE_MIN_W = 160;
    const NODE_HEADER_H = 38;
    const GRID_SIZE = 20;
    const ZOOM_MIN = 0.1;
    const ZOOM_MAX = 2.5;
    const ZOOM_SPEED = 0.0012;
    const CLICK_MOVE_THRESHOLD = 4;
    const INSERT_PREVIEW_DISTANCE = 44;

    // ─── Default themes ───────────────────────────────────────────────────────

    const THEMES = {
        dark: {
            "--fd-bg": "#0f1117",
            "--fd-grid": "#1e2130",
            "--fd-grid-dot": "#2a2f45",
            "--fd-node-bg": "#1a1d2e",
            "--fd-node-border": "#2d3250",
            "--fd-node-border-selected": "#6366f1",
            "--fd-node-header": "#22263a",
            "--fd-node-text": "#e2e8f0",
            "--fd-node-subtext": "#8892a4",
            "--fd-handle": "#6366f1",
            "--fd-handle-hover": "#818cf8",
            "--fd-handle-connected": "#22d3ee",
            "--fd-branch-true": "#22c55e",
            "--fd-branch-false": "#f97316",
            "--fd-insert": "#8b5cf6",
            "--fd-edge": "#4f5a7a",
            "--fd-edge-selected": "#6366f1",
            "--fd-edge-hover": "#818cf8",
            "--fd-shadow": "0 8px 32px rgba(0,0,0,0.45)",
            "--fd-selection-bg": "rgba(99,102,241,0.08)",
            "--fd-selection-border": "#6366f1",
            "--fd-minimap-bg": "#13151f",
            "--fd-controls-bg": "#1a1d2e",
            "--fd-controls-border": "#2d3250",
            "--fd-controls-text": "#e2e8f0",
        },
        light: {
            "--fd-bg": "#f8f9fc",
            "--fd-grid": "#edf0f7",
            "--fd-grid-dot": "#d1d9e8",
            "--fd-node-bg": "#ffffff",
            "--fd-node-border": "#d8dff0",
            "--fd-node-border-selected": "#6366f1",
            "--fd-node-header": "#f4f6fb",
            "--fd-node-text": "#1e2340",
            "--fd-node-subtext": "#6b7280",
            "--fd-handle": "#6366f1",
            "--fd-handle-hover": "#4f46e5",
            "--fd-handle-connected": "#0891b2",
            "--fd-branch-true": "#16a34a",
            "--fd-branch-false": "#ea580c",
            "--fd-insert": "#7c3aed",
            "--fd-edge": "#c5cde0",
            "--fd-edge-selected": "#6366f1",
            "--fd-edge-hover": "#4f46e5",
            "--fd-shadow": "0 4px 20px rgba(0,0,0,0.10)",
            "--fd-selection-bg": "rgba(99,102,241,0.06)",
            "--fd-selection-border": "#6366f1",
            "--fd-minimap-bg": "#edf0f7",
            "--fd-controls-bg": "#ffffff",
            "--fd-controls-border": "#d8dff0",
            "--fd-controls-text": "#1e2340",
        },
    };

    // ─── EdgeRenderer ─────────────────────────────────────────────────────────

    class EdgeRenderer {
        constructor(svg, flow) {
            this.svg = svg;
            this.flow = flow;
            this.edgeEls = new Map(); // edgeId → { group, path, hitPath }
            this.defs = svgEl("defs");
            svg.appendChild(this.defs);
            this._createArrowMarker();

            // Ghost edge (while connecting)
            this.ghostGroup = svgEl("g", { class: "fd-ghost-edge", style: "pointer-events:none" });
            this.ghostPath = svgEl("path", {
                fill: "none",
                stroke: "var(--fd-handle)",
                "stroke-width": "2",
                "stroke-dasharray": "6,4",
                opacity: "0.7",
            });
            this.ghostGroup.appendChild(this.ghostPath);
            svg.appendChild(this.ghostGroup);
        }

        _createArrowMarker() {
            const createMarker = (id, fill) => {
                const marker = svgEl("marker", {
                    id,
                    markerWidth: "10",
                    markerHeight: "10",
                    refX: "9",
                    refY: "3",
                    orient: "auto",
                    markerUnits: "userSpaceOnUse",
                });
                const path = svgEl("path", {
                    d: "M0,0 L0,6 L9,3 z",
                    fill,
                });
                marker.appendChild(path);
                this.defs.appendChild(marker);
            };

            createMarker("fd-arrow", "var(--fd-edge)");
            createMarker("fd-arrow-sel", "var(--fd-edge-selected)");
            createMarker("fd-arrow-true", "var(--fd-branch-true)");
            createMarker("fd-arrow-false", "var(--fd-branch-false)");
            createMarker("fd-arrow-insert", "var(--fd-insert)");
        }

        render(edges, selectedEdgeIds = new Set()) {
            const seen = new Set();

            for (const edge of edges) {
                seen.add(edge.id);
                const coords = this.flow._getEdgeCoords(edge);
                if (!coords) continue;
                const { x1, y1, x2, y2 } = coords;
                const d = bezierPath(x1, y1, x2, y2);
                const isSelected = selectedEdgeIds.has(edge.id);

                if (!this.edgeEls.has(edge.id)) {
                    this._createEdgeEl(edge);
                }
                const { group, path, hitPath, label, deleteBtn } = this.edgeEls.get(edge.id);

                const branch = edge.sourceHandle === "true" || edge.sourceHandle === "false" ? edge.sourceHandle : "";
                const isInsert = this.flow._dropPreview?.edgeId === edge.id;
                const isDimmed = !!this.flow._dragging && this.flow._dragging.hasMoved && !isInsert;
                const stroke = isInsert
                    ? "var(--fd-insert)"
                    : isSelected
                    ? "var(--fd-edge-selected)"
                    : branch === "true"
                    ? "var(--fd-branch-true)"
                    : branch === "false"
                    ? "var(--fd-branch-false)"
                    : edge.style?.stroke || "var(--fd-edge)";
                const strokeW = isInsert ? 3 : isSelected ? 2.5 : edge.style?.strokeWidth || 2;
                const marker = isInsert
                    ? "url(#fd-arrow-insert)"
                    : isSelected
                    ? "url(#fd-arrow-sel)"
                    : branch === "true"
                    ? "url(#fd-arrow-true)"
                    : branch === "false"
                    ? "url(#fd-arrow-false)"
                    : "url(#fd-arrow)";

                group.classList.toggle("fd-edge--selected", isSelected);
                group.classList.toggle("fd-edge--branch-true", branch === "true");
                group.classList.toggle("fd-edge--branch-false", branch === "false");
                group.classList.toggle("fd-edge--insert-target", isInsert);
                group.classList.toggle("fd-edge--dimmed", isDimmed);
                path.setAttribute("d", d);
                path.setAttribute("stroke", stroke);
                path.setAttribute("stroke-width", strokeW);
                path.setAttribute("marker-end", edge.markerEnd !== false ? marker : "");
                hitPath.setAttribute("d", d);

                const mx = (x1 + x2) / 2;
                const my = (y1 + y2) / 2;

                if (edge.label && label) {
                    label.setAttribute("x", mx);
                    label.setAttribute("y", my - 8);
                    label.textContent = edge.label;
                    label.classList.toggle("fd-edge__label--true", branch === "true");
                    label.classList.toggle("fd-edge__label--false", branch === "false");
                }

                if (deleteBtn) {
                    deleteBtn.setAttribute("transform", `translate(${mx}, ${my})`);
                }
            }

            // Remove old
            for (const [id, { group }] of this.edgeEls) {
                if (!seen.has(id)) {
                    group.remove();
                    this.edgeEls.delete(id);
                }
            }
        }

        _createEdgeEl(edge) {
            const group = svgEl("g", { class: "fd-edge", "data-edge-id": edge.id });

            const path = svgEl("path", {
                fill: "none",
                "stroke-linecap": "round",
            });

            // Wide invisible hit area
            const hitPath = svgEl("path", {
                fill: "none",
                stroke: "transparent",
                "stroke-width": "14",
                cursor: "pointer",
                "data-edge-id": edge.id,
                "pointer-events": "stroke",
            });

            group.appendChild(path);
            group.appendChild(hitPath);

            let label = null;
            if (edge.label) {
                label = svgEl("text", {
                    "text-anchor": "middle",
                    fill: "var(--fd-node-subtext)",
                    "font-size": "11",
                    "font-family": "inherit",
                    "pointer-events": "none",
                });
                group.appendChild(label);
            }

            const deleteBtn = svgEl("g", {
                class: "fd-edge-delete",
                "data-edge-delete-id": edge.id,
                "aria-label": "Remove connection",
                role: "button",
                tabindex: "0",
            });
            deleteBtn.appendChild(svgEl("circle", { r: "10" }));
            const deleteIcon = svgEl("text", {
                "text-anchor": "middle",
                "dominant-baseline": "central",
                y: "-0.5",
                "pointer-events": "none",
            });
            deleteIcon.textContent = "×";
            deleteBtn.appendChild(deleteIcon);
            deleteBtn.addEventListener("mousedown", e => {
                e.stopPropagation();
                e.preventDefault();
            });
            deleteBtn.addEventListener("click", e => {
                e.stopPropagation();
                e.preventDefault();
                this.flow._deleteEdgeFromControl(edge.id, e);
            });
            deleteBtn.addEventListener("keydown", e => {
                if (e.key !== "Enter" && e.key !== " ") return;
                e.stopPropagation();
                e.preventDefault();
                this.flow._deleteEdgeFromControl(edge.id, e);
            });
            group.appendChild(deleteBtn);

            this.svg.insertBefore(group, this.ghostGroup);
            this.edgeEls.set(edge.id, { group, path, hitPath, label, deleteBtn });
        }

        updateGhost(x1, y1, x2, y2, visible) {
            if (!visible) {
                this.ghostPath.setAttribute("d", "");
                return;
            }
            this.ghostPath.setAttribute("d", bezierPath(x1, y1, x2, y2));
        }
    }

    // ─── NodeRenderer ─────────────────────────────────────────────────────────

    class NodeRenderer {
        constructor(container, flow) {
            this.container = container;
            this.flow = flow;
            this.nodeEls = new Map(); // nodeId → el
        }

        render(nodes, selectedIds = new Set()) {
            const seen = new Set();

            for (const node of nodes) {
                seen.add(node.id);
                if (!this.nodeEls.has(node.id)) {
                    this._createNodeEl(node);
                }
                this._updateNodeEl(node, selectedIds.has(node.id));
            }

            for (const [id, el] of this.nodeEls) {
                if (!seen.has(id)) {
                    el.remove();
                    this.nodeEls.delete(id);
                }
            }
        }

        _createNodeEl(node) {
            const el = document.createElement("div");
            el.className = "fd-node";
            el.dataset.nodeId = node.id;
            el.style.position = "absolute";

            const typeClass = node.type || "default";
            el.classList.add(`fd-node--${typeClass}`);

            el._fdRenderedHTML = this._renderNodeHTML(node);
            el.innerHTML = el._fdRenderedHTML;
            this._bindNodeHandleEvents(el, node);

            this.container.appendChild(el);
            this.nodeEls.set(node.id, el);
        }

        _bindNodeHandleEvents(el, node) {
            el.querySelectorAll(".fd-handle").forEach(h => {
                h.addEventListener("mousedown", e => {
                    e.stopPropagation();
                    if (this.flow._spacePanning && e.button === 0) {
                        e.preventDefault();
                        this.flow._startPanning(e);
                        return;
                    }
                    this.flow._onHandleMouseDown(e, node.id, h.dataset.handleId, h.dataset.handleType);
                });
                h.addEventListener("mouseenter", () => {
                    h.classList.add("fd-handle--hover");
                    if (h.dataset.handleType === "source") {
                        this.flow._previewHandle(node.id, h.dataset.handleId, true);
                    }
                });
                h.addEventListener("mouseleave", () => {
                    h.classList.remove("fd-handle--hover");
                    if (!this.flow._connecting) {
                        this.flow._previewHandle(node.id, h.dataset.handleId, false);
                    }
                });
            });
        }

        _renderNodeHTML(node) {
            const handles = node.handles || this._defaultHandles(node);
            const sourceHandles = handles.filter(h => h.type === "source");
            const targetHandles = handles.filter(h => h.type === "target");
            const customRenderer = this.flow._options.nodeTypes && this.flow._options.nodeTypes[node.type];
            const customContent = customRenderer
                ? customRenderer(node)
                : (node.content || node.data?.content || "");
            const icon = node.icon ? `<span class="fd-node__icon">${escapeHtml(node.icon)}</span>` : "";
            const badge = node.badge ? `<span class="fd-node__badge">${escapeHtml(node.badge)}</span>` : "";

            const renderHandles = (list, side) =>
                list
                    .map(
                        (h, i) => `
          <div class="fd-handle fd-handle--${h.type} fd-handle--${side}"
            data-handle-id="${h.id}"
            data-handle-type="${h.type}"
            data-node-id="${node.id}"
            data-handle-branch="${escapeHtml(h.branch || h.id || "")}"
            style="top: ${h.position != null ? h.position + "%" : (100 / (list.length + 1)) * (i + 1) + "%"}"
            title="${escapeHtml(h.label || h.id)}">
            ${h.type === "source" ? `<span class="fd-handle__plus">+</span>` : ""}
          </div>`,
                    )
                    .join("");

            return `
        <div class="fd-node__handles fd-node__handles--left">
          ${renderHandles(targetHandles, "left")}
        </div>
        <div class="fd-node__inner">
          <div class="fd-node__header">
            ${icon}
            <span class="fd-node__title">${escapeHtml(node.label || node.data?.label || "Node")}</span>
            ${badge}
          </div>
          ${customContent ? `<div class="fd-node__content">${customContent}</div>` : ""}
        </div>
        <div class="fd-node__handles fd-node__handles--right">
          ${renderHandles(sourceHandles, "right")}
        </div>
      `;
        }

        _defaultHandles(node) {
            if (node.type === "input") {
                return [{ id: "out", type: "source" }];
            }
            if (node.type === "output") {
                return [{ id: "in", type: "target" }];
            }
            return [
                { id: "in", type: "target" },
                { id: "out", type: "source" },
            ];
        }

        _updateNodeEl(node, isSelected) {
            const el = this.nodeEls.get(node.id);
            if (!el) return;

            const nextHTML = this._renderNodeHTML(node);
            if (el._fdRenderedHTML !== nextHTML) {
                el._fdRenderedHTML = nextHTML;
                el.innerHTML = nextHTML;
                this._bindNodeHandleEvents(el, node);
            }

            el.style.transform = `translate(${node.position.x}px, ${node.position.y}px)`;
            el.style.width = node.width ? node.width + "px" : "";

            el.classList.toggle("fd-node--selected", isSelected);
            el.classList.toggle("fd-node--dragging", !!node._dragging);
            el.classList.toggle("fd-node--drop-invalid", this.flow._isNodeDimmedDuringDrag(node.id));

            const nudge = this.flow._nodeInsertionNudge(node.id);
            el.style.translate = nudge ? `${nudge.x}px ${nudge.y}px` : "";

            // Update title if changed
            const titleEl = el.querySelector(".fd-node__title");
            if (titleEl && titleEl.textContent !== node.label) {
                titleEl.textContent = node.label || "Node";
            }
        }

        getHandlePosition(nodeId, handleId, handleType) {
            const node = this.flow._getNode(nodeId);
            const nodeEl = this.nodeEls.get(nodeId);
            if (!node || !nodeEl) return null;

            const handleEl = nodeEl.querySelector(
                `.fd-handle[data-handle-id="${handleId}"][data-handle-type="${handleType}"]`,
            );
            if (!handleEl) return null;

            // IMPORTANT: edges live in the same transformed viewport as nodes.
            // Do not calculate edge endpoints from getBoundingClientRect(), because
            // those values are already affected by zoom/pan and can drift after transforms.
            // Instead, calculate in canvas coordinates from the node model + DOM size.
            const width = node.width || nodeEl.offsetWidth || NODE_MIN_W;
            const height = node.height || nodeEl.offsetHeight || NODE_HEADER_H;
            const side = handleEl.classList.contains("fd-handle--left") ? "left" : "right";

            let topPercent = 50;
            const inlineTop = handleEl.style.top || "";
            if (inlineTop.endsWith("%")) {
                topPercent = parseFloat(inlineTop) || 50;
            } else {
                const rect = handleEl.getBoundingClientRect();
                const nodeRect = nodeEl.getBoundingClientRect();
                if (nodeRect.height) topPercent = ((rect.top + rect.height / 2 - nodeRect.top) / nodeRect.height) * 100;
            }

            return {
                x: node.position.x + (side === "left" ? 0 : width),
                y: node.position.y + (height * topPercent) / 100,
            };
        }
    }

    // ─── MiniMap ──────────────────────────────────────────────────────────────

    class MiniMap {
        constructor(container, flow) {
            this.flow = flow;
            this.el = document.createElement("div");
            this.el.className = "fd-minimap";
            this.canvas = document.createElement("canvas");
            this.canvas.width = 200;
            this.canvas.height = 130;
            this.el.appendChild(this.canvas);
            container.appendChild(this.el);
            this.ctx = this.canvas.getContext("2d");

            this.el.addEventListener("click", e => this._onClick(e));
        }

        render(nodes, edges, viewport, containerSize) {
            const ctx = this.ctx;
            const W = this.canvas.width;
            const H = this.canvas.height;
            ctx.clearRect(0, 0, W, H);

            if (!nodes.length) return;

            // Compute bounding box of all nodes
            let minX = Infinity,
                minY = Infinity,
                maxX = -Infinity,
                maxY = -Infinity;
            for (const n of nodes) {
                minX = Math.min(minX, n.position.x);
                minY = Math.min(minY, n.position.y);
                maxX = Math.max(maxX, n.position.x + (n.width || 180));
                maxY = Math.max(maxY, n.position.y + 60);
            }

            const pad = 30;
            const scaleX = (W - pad * 2) / (maxX - minX || 1);
            const scaleY = (H - pad * 2) / (maxY - minY || 1);
            const s = Math.min(scaleX, scaleY, 1);

            const toMM = (x, y) => ({
                x: (x - minX) * s + pad,
                y: (y - minY) * s + pad,
            });

            // Draw edges
            ctx.strokeStyle = "rgba(99,102,241,0.35)";
            ctx.lineWidth = 1;
            for (const edge of edges) {
                const coords = this.flow._getEdgeCoords(edge);
                if (!coords) continue;
                const { x1, y1, x2, y2 } = coords;
                const p1 = toMM(x1, y1);
                const p2 = toMM(x2, y2);
                ctx.beginPath();
                ctx.moveTo(p1.x, p1.y);
                ctx.lineTo(p2.x, p2.y);
                ctx.stroke();
            }

            // Draw nodes
            for (const n of nodes) {
                const p = toMM(n.position.x, n.position.y);
                const w = (n.width || 180) * s;
                const h = 40 * s;
                ctx.fillStyle = n._selected ? "rgba(99,102,241,0.7)" : "rgba(99,102,241,0.25)";
                ctx.strokeStyle = n._selected ? "#6366f1" : "rgba(99,102,241,0.4)";
                ctx.lineWidth = 1;
                ctx.beginPath();
                ctx.roundRect(p.x, p.y, Math.max(w, 4), Math.max(h, 4), 2);
                ctx.fill();
                ctx.stroke();
            }

            // Viewport rect
            const vx = (-viewport.x / viewport.scale - minX) * s + pad;
            const vy = (-viewport.y / viewport.scale - minY) * s + pad;
            const vw = (containerSize.w / viewport.scale) * s;
            const vh = (containerSize.h / viewport.scale) * s;

            ctx.strokeStyle = "rgba(99,102,241,0.8)";
            ctx.lineWidth = 1.5;
            ctx.setLineDash([3, 2]);
            ctx.beginPath();
            ctx.rect(vx, vy, vw, vh);
            ctx.stroke();
            ctx.setLineDash([]);

            // Store for click-to-pan
            this._mmScale = s;
            this._mmOffset = { x: minX, y: minY };
        }

        _onClick(e) {
            if (!this._mmScale) return;
            const rect = this.canvas.getBoundingClientRect();
            const mx = e.clientX - rect.left;
            const my = e.clientY - rect.top;
            const pad = 30;
            const wx = (mx - pad) / this._mmScale + this._mmOffset.x;
            const wy = (my - pad) / this._mmScale + this._mmOffset.y;
            this.flow.panTo(
                -wx * this.flow.viewport.scale + this.flow._containerSize().w / 2,
                -wy * this.flow.viewport.scale + this.flow._containerSize().h / 2,
            );
        }
    }

    // ─── Controls ─────────────────────────────────────────────────────────────

    class Controls {
        constructor(container, flow) {
            this.flow = flow;
            this.el = document.createElement("div");
            this.el.className = "fd-controls";
            this.el.innerHTML = `
        <button class="fd-controls__btn" data-action="zoom-in" title="Zoom In">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5">
            <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
            <line x1="11" y1="8" x2="11" y2="14"/><line x1="8" y1="11" x2="14" y2="11"/>
          </svg>
        </button>
        <button class="fd-controls__btn" data-action="zoom-out" title="Zoom Out">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5">
            <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
            <line x1="8" y1="11" x2="14" y2="11"/>
          </svg>
        </button>
        <button class="fd-controls__btn" data-action="fit" title="Fit View">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5">
            <polyline points="15 3 21 3 21 9"/><polyline points="9 21 3 21 3 15"/>
            <line x1="21" y1="3" x2="14" y2="10"/><line x1="3" y1="21" x2="10" y2="14"/>
          </svg>
        </button>
        <button class="fd-controls__btn" data-action="lock" title="Toggle Lock">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5">
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
            <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
          </svg>
        </button>
      `;
            container.appendChild(this.el);

            this.el.addEventListener("click", e => {
                const btn = e.target.closest("[data-action]");
                if (!btn) return;
                const action = btn.dataset.action;
                if (action === "zoom-in") flow.zoomIn();
                if (action === "zoom-out") flow.zoomOut();
                if (action === "fit") flow.fitView();
                if (action === "lock") flow.toggleLock();
            });
        }

        setLocked(locked) {
            const btn = this.el.querySelector("[data-action=\"lock\"]");
            btn.classList.toggle("fd-controls__btn--active", locked);
        }
    }

    // ─── FlowDesigner ─────────────────────────────────────────────────────────

    class FlowDesigner {
        constructor(container, options = {}) {
            if (typeof container === "string") {
                container = document.querySelector(container);
            }
            if (!container) throw new Error("FlowDesigner: container not found");

            this._container = container;
            this._options = Object.assign(
                {
                    theme: "dark",
                    gridType: "dots", // 'dots' | 'lines' | 'none'
                    snapToGrid: false,
                    snapSize: GRID_SIZE,
                    multiSelect: true,
                    editable: true,
                    onNodeClick: null,
                    onEdgeClick: null,
                    onEdgeDelete: null,
                    onConnect: null,
                    onHandleClick: null,
                    onNodeDragStop: null,
                    onEdgeDrop: null,
                    onChange: null,
                    onSelectionChange: null,
                    defaultEdgeOptions: {},
                    nodeTypes: {},
                    allowDblClickAdd: true,
                    allowDelete: true,
                    allowEdgeDelete: true,
                    defaultZoom: 1,
                },
                options,
            );

            this.nodes = [];
            this.edges = [];
            this.viewport = { x: 0, y: 0, scale: clamp(Number(this._options.defaultZoom) || 1, ZOOM_MIN, ZOOM_MAX) };

            this._selectedNodeIds = new Set();
            this._selectedEdgeIds = new Set();
            this._locked = false;

            // Interaction state
            this._dragging = null; // { nodeIds, startPositions, startMouse }
            this._panning = null; // { startMouse, startViewport }
            this._connecting = null; // { sourceNodeId, handleId, startPos }
            this._dropPreview = null; // { edgeId, x, y }
            this._selecting = null; // { startX, startY }
            this._spacePanning = false;

            this._init();
        }

        // ── Initialization ──────────────────────────────────────────────────────

        _init() {
            this._injectStyles();
            this._buildDOM();
            this._applyTheme(this._options.theme);
            this._bindEvents();
        }

        _injectStyles() {
            if (document.getElementById("fd-styles")) return;
            const style = document.createElement("style");
            style.id = "fd-styles";
            style.textContent = FlowDesigner.CSS;
            document.head.appendChild(style);
        }

        _buildDOM() {
            this._container.classList.add("fd-container");

            // Root
            this._root = document.createElement("div");
            this._root.className = "fd-root";
            this._container.appendChild(this._root);

            // Background (SVG grid)
            this._bgSvg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
            this._bgSvg.setAttribute("class", "fd-background");
            this._bgSvg.style.cssText = "position:absolute;inset:0;width:100%;height:100%;pointer-events:none";
            this._root.appendChild(this._bgSvg);
            this._initBackground();

            // Viewport transform layer
            this._viewport = document.createElement("div");
            this._viewport.className = "fd-viewport";
            this._root.appendChild(this._viewport);

            // Edge SVG layer (inside viewport)
            this._edgeSvg = svgEl("svg", {
                class: "fd-edge-layer",
                style: "position:absolute;overflow:visible;pointer-events:visiblePainted",
                width: "1",
                height: "1",
            });
            this._viewport.appendChild(this._edgeSvg);
            this._edgeRenderer = new EdgeRenderer(this._edgeSvg, this);

            // Node layer
            this._nodeLayer = document.createElement("div");
            this._nodeLayer.className = "fd-node-layer";
            this._viewport.appendChild(this._nodeLayer);
            this._nodeRenderer = new NodeRenderer(this._nodeLayer, this);

            this._insertPlaceholder = document.createElement("div");
            this._insertPlaceholder.className = "fd-insert-placeholder";
            this._insertPlaceholder.innerHTML =
                `<span class="fd-insert-placeholder__icon">+</span><span>Drop to insert</span>`;
            this._insertPlaceholder.style.display = "none";
            this._viewport.appendChild(this._insertPlaceholder);

            // Selection box
            this._selectionBox = document.createElement("div");
            this._selectionBox.className = "fd-selection-box";
            this._selectionBox.style.display = "none";
            this._root.appendChild(this._selectionBox);

            // Controls
            this._controls = new Controls(this._root, this);

            // MiniMap
            if (this._options.minimap !== false) {
                this._minimap = new MiniMap(this._root, this);
            }
        }

        _initBackground() {
            const defs = svgEl("defs");
            const pattern = svgEl("pattern", {
                id: "fd-grid-pattern",
                x: "0",
                y: "0",
                width: GRID_SIZE,
                height: GRID_SIZE,
                patternUnits: "userSpaceOnUse",
            });
            this._bgDot = svgEl("circle", { cx: "0.5", cy: "0.5", r: "0.8", fill: "var(--fd-grid-dot)" });
            pattern.appendChild(this._bgDot);
            defs.appendChild(pattern);
            this._bgSvg.appendChild(defs);

            this._bgRect = svgEl("rect", {
                width: "100%",
                height: "100%",
                fill: "url(#fd-grid-pattern)",
            });
            this._bgSvg.appendChild(this._bgRect);
        }

        _bindEvents() {
            const root = this._root;
            this._boundEvents = {
                wheel: e => this._onWheel(e),
                mousedown: e => this._onMouseDown(e),
                mousemove: e => this._onMouseMove(e),
                mouseup: e => this._onMouseUp(e),
                dblclick: e => this._onDblClick(e),
                keydown: e => this._onKeyDown(e),
                keyup: e => this._onKeyUp(e),
                contextmenu: e => e.preventDefault(),
                mouseover: e => {
                    const h = e.target.closest(".fd-handle");
                    if (h && this._connecting) {
                        h.classList.add("fd-handle--connectable");
                    }
                },
                mouseout: e => {
                    const h = e.target.closest(".fd-handle");
                    if (h) h.classList.remove("fd-handle--connectable");
                },
            };

            // Wheel zoom
            root.addEventListener("wheel", this._boundEvents.wheel, { passive: false });

            // Mouse events
            root.addEventListener("mousedown", this._boundEvents.mousedown);
            window.addEventListener("mousemove", this._boundEvents.mousemove);
            window.addEventListener("mouseup", this._boundEvents.mouseup);

            // Double-click to add node
            root.addEventListener("dblclick", this._boundEvents.dblclick);

            // Keyboard
            document.addEventListener("keydown", this._boundEvents.keydown);
            document.addEventListener("keyup", this._boundEvents.keyup);

            // Context menu
            root.addEventListener("contextmenu", this._boundEvents.contextmenu);

            // Handle hover for connection targets
            root.addEventListener("mouseover", this._boundEvents.mouseover);
            root.addEventListener("mouseout", this._boundEvents.mouseout);
        }

        // ── Events ──────────────────────────────────────────────────────────────

        _onWheel(e) {
            e.preventDefault();
            if (this._locked) return;

            const rect = this._root.getBoundingClientRect();
            const mouseX = e.clientX - rect.left;
            const mouseY = e.clientY - rect.top;

            const delta = -e.deltaY * ZOOM_SPEED;
            const newScale = clamp(this.viewport.scale * (1 + delta), ZOOM_MIN, ZOOM_MAX);
            const factor = newScale / this.viewport.scale;

            this.viewport.x = mouseX - factor * (mouseX - this.viewport.x);
            this.viewport.y = mouseY - factor * (mouseY - this.viewport.y);
            this.viewport.scale = newScale;

            this._applyViewport();
            this._updateBackground();
            this._renderAll();
        }

        _onMouseDown(e) {
            if (e.button !== 0 && e.button !== 1) return;

            if (this._spacePanning && e.button === 0) {
                e.preventDefault();
                this._startPanning(e);
                return;
            }

            const nodeEl = e.target.closest(".fd-node");
            const edgeEl = e.target.closest("[data-edge-id]");
            const handleEl = e.target.closest(".fd-handle");

            if (handleEl) return; // handled by NodeRenderer

            if (nodeEl) {
                const nodeId = nodeEl.dataset.nodeId;
                this._onNodeMouseDown(e, nodeId);
                return;
            }

            if (edgeEl) {
                const edgeId = edgeEl.dataset.edgeId;
                this._onEdgeClick(edgeId, e);
                return;
            }

            // Pan or selection
            if (e.button === 1 || e.altKey || e.button === 0) {
                if (e.button === 1 || e.altKey) {
                    this._startPanning(e);
                } else {
                    // Start selection box
                    if (!this._locked) {
                        this._startSelecting(e);
                    } else {
                        this._startPanning(e);
                    }
                    // Deselect
                    if (!e.shiftKey) this._clearSelection();
                }
            }
        }

        _onMouseMove(e) {
            if (this._panning) {
                const dx = e.clientX - this._panning.startMouse.x;
                const dy = e.clientY - this._panning.startMouse.y;
                this.viewport.x = this._panning.startViewport.x + dx;
                this.viewport.y = this._panning.startViewport.y + dy;
                this._applyViewport();
                this._updateBackground();
                this._renderAll();
                return;
            }

            if (this._dragging) {
                const movedDistance = Math.hypot(
                    e.clientX - this._dragging.startMouse.x,
                    e.clientY - this._dragging.startMouse.y,
                );
                if (!this._dragging.hasMoved && movedDistance <= CLICK_MOVE_THRESHOLD) {
                    return;
                }
                this._dragging.hasMoved = true;

                const dx = (e.clientX - this._dragging.startMouse.x) / this.viewport.scale;
                const dy = (e.clientY - this._dragging.startMouse.y) / this.viewport.scale;

                for (const nodeId of this._dragging.nodeIds) {
                    const node = this._getNode(nodeId);
                    const startPos = this._dragging.startPositions.get(nodeId);
                    if (!node || !startPos) continue;

                    let nx = startPos.x + dx;
                    let ny = startPos.y + dy;

                    if (this._options.snapToGrid) {
                        const snap = this._options.snapSize;
                        nx = Math.round(nx / snap) * snap;
                        ny = Math.round(ny / snap) * snap;
                    }

                    node.position = { x: nx, y: ny };
                    node._dragging = true;
                }

                this._updateDropPreview(e.clientX, e.clientY);
                this._renderAll();
                return;
            }

            if (this._connecting) {
                const pos = this._screenToCanvas(e.clientX, e.clientY);
                const { x1, y1 } = this._connecting;
                this._connecting.hasMoved = Math.hypot(
                    e.clientX - this._connecting.startMouse.x,
                    e.clientY - this._connecting.startMouse.y,
                ) > CLICK_MOVE_THRESHOLD;
                this._edgeRenderer.updateGhost(x1, y1, pos.x, pos.y, true);
                return;
            }

            if (this._selecting) {
                const pos = this._screenToCanvas(e.clientX, e.clientY);
                const sx = Math.min(this._selecting.startCanvas.x, pos.x);
                const sy = Math.min(this._selecting.startCanvas.y, pos.y);
                const sw = Math.abs(pos.x - this._selecting.startCanvas.x);
                const sh = Math.abs(pos.y - this._selecting.startCanvas.y);

                // Convert back to screen for the selection box div
                const p1 = this._canvasToScreen(sx, sy);
                const p2 = this._canvasToScreen(sx + sw, sy + sh);
                const rootRect = this._root.getBoundingClientRect();

                this._selectionBox.style.display = "block";
                this._selectionBox.style.left = p1.x - rootRect.left + "px";
                this._selectionBox.style.top = p1.y - rootRect.top + "px";
                this._selectionBox.style.width = p2.x - p1.x + "px";
                this._selectionBox.style.height = p2.y - p1.y + "px";

                // Highlight nodes in selection
                for (const node of this.nodes) {
                    const inBox = node.position.x + (node.width || 180) >= sx
                        && node.position.x <= sx + sw
                        && node.position.y + 60 >= sy
                        && node.position.y <= sy + sh;
                    if (inBox) this._selectedNodeIds.add(node.id);
                    else if (!e.shiftKey) this._selectedNodeIds.delete(node.id);
                }
                this._renderAll();
                return;
            }
        }

        _onMouseUp(e) {
            if (this._dragging) {
                const draggedNodeId = this._dragging.clickedNodeId;
                const isClick = !this._dragging.hasMoved;

                for (const nodeId of this._dragging.nodeIds) {
                    const node = this._getNode(nodeId);
                    if (node) delete node._dragging;
                }
                const dropPreview = this._dropPreview;
                if (!isClick) {
                    this._options.onNodeDragStop && this._options.onNodeDragStop(
                        this._dragging.nodeIds.map(id => this._getNode(id)),
                    );
                    if (dropPreview) {
                        const edge = this._getEdge(dropPreview.edgeId);
                        const nodes = this._dragging.nodeIds.map(id => this._getNode(id)).filter(Boolean);
                        this._options.onEdgeDrop && this._options.onEdgeDrop({ edge, nodes, position: dropPreview });
                        emit(this._container, "fd:edgedrop", { edge, nodes, position: dropPreview });
                    }
                    this._fireChange();
                }
                this._dragging = null;
                this._clearDropPreview();
                if (isClick) {
                    const node = this._getNode(draggedNodeId);
                    this._options.onNodeClick && this._options.onNodeClick(node, e);
                    emit(this._container, "fd:nodeclick", { node });
                }
                this._renderAll();
            }

            if (this._panning) {
                this._panning = null;
                this._root.style.cursor = "";
            }

            if (this._connecting) {
                const targetHandle = e.target.closest(".fd-handle[data-handle-type=\"target\"]");
                if (targetHandle) {
                    const targetNodeId = targetHandle.closest(".fd-node").dataset.nodeId;
                    const targetHandleId = targetHandle.dataset.handleId;
                    this._createEdge(
                        this._connecting.sourceNodeId,
                        this._connecting.handleId,
                        targetNodeId,
                        targetHandleId,
                    );
                } else if (!this._connecting.hasMoved) {
                    this._emitHandleClick(this._connecting.sourceNodeId, this._connecting.handleId, e);
                }
                this._edgeRenderer.updateGhost(0, 0, 0, 0, false);
                this._connecting = null;
                this._previewHandle("", "", false);
                this._root.classList.remove("fd-root--connecting");
            }

            if (this._selecting) {
                this._selecting = null;
                this._selectionBox.style.display = "none";
                this._renderAll();
            }
        }

        _onDblClick(e) {
            if (!this._options.editable) return;
            const nodeEl = e.target.closest(".fd-node");
            if (nodeEl) {
                const node = this._getNode(nodeEl.dataset.nodeId);
                emit(this._container, "fd:nodedblclick", { node });
                return;
            }
            if (!this._options.allowDblClickAdd) return;
            if (e.target.closest(".fd-controls") || e.target.closest(".fd-minimap")) return;

            // Add node at click position
            const pos = this._screenToCanvas(e.clientX, e.clientY);
            const node = this.addNode({
                label: "New Node",
                position: { x: pos.x - 90, y: pos.y - 20 },
            });
            emit(this._container, "fd:nodeadd", { node });
        }

        _onKeyDown(e) {
            if (!this._root.contains(document.activeElement) && document.activeElement !== document.body) return;

            if ((e.key === " " || e.code === "Space") && !this._isTextInput(document.activeElement)) {
                e.preventDefault();
                this._setSpacePanning(true);
                return;
            }

            if ((e.key === "Delete" || e.key === "Backspace") && this._options.editable) {
                if (this._isTextInput(document.activeElement)) return;
                if (!this._options.allowDelete) return;
                this._deleteSelected();
            }

            if (e.key === "a" && (e.ctrlKey || e.metaKey)) {
                e.preventDefault();
                this.selectAll();
            }

            if (e.key === "Escape") {
                this._clearSelection();
                this._renderAll();
            }
        }

        _onKeyUp(e) {
            if (e.key === " " || e.code === "Space") {
                this._setSpacePanning(false);
            }
        }

        _onNodeMouseDown(e, nodeId) {
            if (this._locked) return;
            e.stopPropagation();

            const isSelected = this._selectedNodeIds.has(nodeId);

            if (e.shiftKey && this._options.multiSelect) {
                if (isSelected) this._selectedNodeIds.delete(nodeId);
                else this._selectedNodeIds.add(nodeId);
            } else {
                if (!isSelected) {
                    this._clearSelection();
                    this._selectedNodeIds.add(nodeId);
                }
            }

            const nodeIds = [...this._selectedNodeIds];
            const startPositions = new Map();
            for (const id of nodeIds) {
                const n = this._getNode(id);
                if (n) startPositions.set(id, { ...n.position });
            }

            this._dragging = {
                nodeIds,
                startPositions,
                startMouse: { x: e.clientX, y: e.clientY },
                clickedNodeId: nodeId,
                hasMoved: false,
            };

            this._renderAll();
        }

        _onEdgeClick(edgeId, e) {
            if (e.target.closest(".fd-edge-delete")) return;
            if (!e.shiftKey) this._clearSelection();
            this._selectedEdgeIds.add(edgeId);
            const edge = this._getEdge(edgeId);
            this._options.onEdgeClick && this._options.onEdgeClick(edge, e);
            emit(this._container, "fd:edgeclick", { edge });
            this._renderAll();
        }

        _onHandleMouseDown(e, nodeId, handleId, handleType) {
            if (!this._options.editable || handleType !== "source") return;
            e.stopPropagation();
            e.preventDefault();

            const pos = this._nodeRenderer.getHandlePosition(nodeId, handleId, "source");
            if (!pos) return;

            this._connecting = {
                sourceNodeId: nodeId,
                handleId,
                x1: pos.x,
                y1: pos.y,
                startMouse: { x: e.clientX, y: e.clientY },
                hasMoved: false,
            };
            this._root.classList.add("fd-root--connecting");
        }

        _deleteEdgeFromControl(edgeId, originalEvent) {
            if (!this._options.editable || this._options.allowEdgeDelete === false) return;
            const edge = this._getEdge(edgeId);
            if (!edge) return;

            if (this._options.onEdgeDelete) {
                const result = this._options.onEdgeDelete(edge, originalEvent);
                if (result === false) return;
            }

            this.edges = this.edges.filter(e => e.id !== edgeId);
            this._selectedEdgeIds.delete(edgeId);
            this._fireChange();
            this._renderAll();
            emit(this._container, "fd:edgedelete", { edge });
        }

        // ── Core helpers ────────────────────────────────────────────────────────

        _startPanning(e) {
            this._panning = {
                startMouse: { x: e.clientX, y: e.clientY },
                startViewport: { ...this.viewport },
            };
            this._root.style.cursor = "grabbing";
        }

        _setSpacePanning(isPanning) {
            if (this._spacePanning === isPanning) return;
            this._spacePanning = isPanning;
            this._root.classList.toggle("fd-root--space-panning", isPanning);
        }

        _previewHandle(nodeId, handleId, visible) {
            if (!visible) {
                this._edgeRenderer.updateGhost(0, 0, 0, 0, false);
                return;
            }
            const pos = this._nodeRenderer.getHandlePosition(nodeId, handleId, "source");
            if (!pos) return;
            this._edgeRenderer.updateGhost(pos.x, pos.y, pos.x + 72, pos.y, true);
        }

        _isTextInput(el) {
            if (!el) return false;
            return el.tagName === "INPUT"
                || el.tagName === "TEXTAREA"
                || el.tagName === "SELECT"
                || el.isContentEditable;
        }

        _startSelecting(e) {
            const pos = this._screenToCanvas(e.clientX, e.clientY);
            this._selecting = { startCanvas: pos };
        }

        _clearSelection() {
            this._selectedNodeIds.clear();
            this._selectedEdgeIds.clear();
            this._emitSelection();
        }

        _deleteSelected() {
            for (const id of this._selectedEdgeIds) {
                this.edges = this.edges.filter(e => e.id !== id);
            }
            for (const id of this._selectedNodeIds) {
                // Remove connected edges
                this.edges = this.edges.filter(e => e.source !== id && e.target !== id);
                this.nodes = this.nodes.filter(n => n.id !== id);
            }
            this._clearSelection();
            this._fireChange();
            this._renderAll();
        }

        _createEdge(sourceNodeId, sourceHandleId, targetNodeId, targetHandleId) {
            // Prevent self-loops
            if (sourceNodeId === targetNodeId) return;
            // Prevent duplicate
            const exists = this.edges.find(
                e => e.source === sourceNodeId
                    && e.sourceHandle === sourceHandleId
                    && e.target === targetNodeId
                    && e.targetHandle === targetHandleId,
            );
            if (exists) return;

            const edge = Object.assign({}, this._options.defaultEdgeOptions, {
                id: "e_" + uid(),
                source: sourceNodeId,
                sourceHandle: sourceHandleId,
                target: targetNodeId,
                targetHandle: targetHandleId,
            });

            // User callback can modify/cancel
            if (this._options.onConnect) {
                const result = this._options.onConnect(edge);
                if (result === false) return;
                if (result && typeof result === "object") Object.assign(edge, result);
            }

            this.edges.push(edge);
            this._fireChange();
            this._renderAll();
            emit(this._container, "fd:connect", { edge });
        }

        _emitHandleClick(nodeId, handleId, originalEvent) {
            const node = this._getNode(nodeId);
            const detail = { node, nodeId, handleId, originalEvent };
            this._options.onHandleClick && this._options.onHandleClick(detail);
            emit(this._container, "fd:handleclick", detail);
        }

        _updateDropPreview(clientX, clientY) {
            if (!this._dragging?.hasMoved) return;
            const pos = this._screenToCanvas(clientX, clientY);
            const draggedIds = new Set(this._dragging.nodeIds);
            let best = null;

            for (const edge of this.edges) {
                if (draggedIds.has(edge.source) || draggedIds.has(edge.target)) {
                    continue;
                }
                const coords = this._getEdgeCoords(edge);
                if (!coords) continue;
                const distance = this._distanceToSegment(pos.x, pos.y, coords.x1, coords.y1, coords.x2, coords.y2);
                if (distance > INSERT_PREVIEW_DISTANCE) {
                    continue;
                }
                if (!best || distance < best.distance) {
                    best = {
                        edgeId: edge.id,
                        x: (coords.x1 + coords.x2) / 2,
                        y: (coords.y1 + coords.y2) / 2,
                        distance,
                    };
                }
            }

            this._dropPreview = best;
            this._root.classList.toggle("fd-root--drop-preview", !!best);
            this._positionInsertPlaceholder();
        }

        _clearDropPreview() {
            this._dropPreview = null;
            this._root.classList.remove("fd-root--drop-preview");
            if (this._insertPlaceholder) {
                this._insertPlaceholder.style.display = "none";
            }
        }

        _positionInsertPlaceholder() {
            if (!this._insertPlaceholder) return;
            if (!this._dropPreview) {
                this._insertPlaceholder.style.display = "none";
                return;
            }
            this._insertPlaceholder.style.display = "flex";
            this._insertPlaceholder.style.transform = `translate(${this._dropPreview.x - 58}px, ${
                this._dropPreview.y - 20
            }px)`;
        }

        _distanceToSegment(px, py, x1, y1, x2, y2) {
            const dx = x2 - x1;
            const dy = y2 - y1;
            if (!dx && !dy) return Math.hypot(px - x1, py - y1);
            const t = clamp(((px - x1) * dx + (py - y1) * dy) / (dx * dx + dy * dy), 0, 1);
            return Math.hypot(px - (x1 + t * dx), py - (y1 + t * dy));
        }

        _nodeInsertionNudge(nodeId) {
            if (!this._dropPreview) return null;
            const edge = this._getEdge(this._dropPreview.edgeId);
            if (!edge) return null;
            if (edge.source === nodeId) return { x: -12, y: 0 };
            if (edge.target === nodeId) return { x: 12, y: 0 };
            return null;
        }

        _isNodeDimmedDuringDrag(nodeId) {
            if (!this._dragging?.hasMoved) return false;
            if (this._dragging.nodeIds.includes(nodeId)) return false;
            const edge = this._dropPreview ? this._getEdge(this._dropPreview.edgeId) : null;
            return !(edge && (edge.source === nodeId || edge.target === nodeId));
        }

        _getNode(id) {
            return this.nodes.find(n => n.id === id);
        }

        _getEdge(id) {
            return this.edges.find(e => e.id === id);
        }

        _getEdgeCoords(edge) {
            const sourcePos = this._nodeRenderer.getHandlePosition(edge.source, edge.sourceHandle, "source");
            const targetPos = this._nodeRenderer.getHandlePosition(edge.target, edge.targetHandle, "target");
            if (!sourcePos || !targetPos) return null;
            return { x1: sourcePos.x, y1: sourcePos.y, x2: targetPos.x, y2: targetPos.y };
        }

        _emitSelection() {
            const selection = {
                nodes: [...this._selectedNodeIds].map(id => this._getNode(id)).filter(Boolean),
                edges: [...this._selectedEdgeIds].map(id => this._getEdge(id)).filter(Boolean),
                nodeIds: [...this._selectedNodeIds],
                edgeIds: [...this._selectedEdgeIds],
            };
            this._options.onSelectionChange && this._options.onSelectionChange(selection);
            emit(this._container, "fd:selectionchange", selection);
        }

        _screenToCanvas(sx, sy) {
            const rect = this._root.getBoundingClientRect();
            return {
                x: (sx - rect.left - this.viewport.x) / this.viewport.scale,
                y: (sy - rect.top - this.viewport.y) / this.viewport.scale,
            };
        }

        _canvasToScreen(cx, cy) {
            const rect = this._root.getBoundingClientRect();
            return {
                x: cx * this.viewport.scale + this.viewport.x + rect.left,
                y: cy * this.viewport.scale + this.viewport.y + rect.top,
            };
        }

        _containerSize() {
            return { w: this._root.clientWidth, h: this._root.clientHeight };
        }

        _applyViewport() {
            this._viewport.style.transform =
                `translate(${this.viewport.x}px, ${this.viewport.y}px) scale(${this.viewport.scale})`;
        }

        _updateBackground() {
            if (this._options.gridType === "none") return;
            const s = this.viewport.scale;
            const gx = this.viewport.x % (GRID_SIZE * s);
            const gy = this.viewport.y % (GRID_SIZE * s);

            const pattern = this._bgSvg.querySelector("pattern");
            pattern.setAttribute("x", gx);
            pattern.setAttribute("y", gy);
            pattern.setAttribute("width", GRID_SIZE * s);
            pattern.setAttribute("height", GRID_SIZE * s);
        }

        _renderAll() {
            // Mark selected for minimap
            for (const n of this.nodes) {
                n._selected = this._selectedNodeIds.has(n.id);
            }

            this._nodeRenderer.render(this.nodes, this._selectedNodeIds);
            this._edgeRenderer.render(this.edges, this._selectedEdgeIds);
            this._positionInsertPlaceholder();

            if (this._minimap) {
                this._minimap.render(this.nodes, this.edges, this.viewport, this._containerSize());
            }
        }

        _applyTheme(theme) {
            const vars = typeof theme === "string" ? THEMES[theme] : theme;
            if (!vars) return;
            for (const [k, v] of Object.entries(vars)) {
                this._container.style.setProperty(k, v);
            }
        }

        _fireChange() {
            this._options.onChange && this._options.onChange({
                nodes: deepClone(this.nodes.map(n => {
                    const c = { ...n };
                    delete c._dragging;
                    delete c._selected;
                    return c;
                })),
                edges: deepClone(this.edges),
            });
            emit(this._container, "fd:change", { nodes: this.nodes, edges: this.edges });
        }

        // ── Public API ──────────────────────────────────────────────────────────

        /**
         * Set the full graph
         */
        setGraph({ nodes = [], edges = [] }) {
            this.nodes = deepClone(nodes);
            this.edges = deepClone(edges);
            this._clearSelection();
            this._renderAll();
            return this;
        }

        /**
         * Get the full graph (clean copy)
         */
        getGraph() {
            return {
                nodes: deepClone(this.nodes.map(n => {
                    const c = { ...n };
                    delete c._dragging;
                    delete c._selected;
                    return c;
                })),
                edges: deepClone(this.edges),
            };
        }

        /** React Flow-like aliases */
        setNodes(nodes = []) {
            this.nodes = deepClone(nodes);
            this._clearSelection();
            this._renderAll();
            this._fireChange();
            return this;
        }

        setEdges(edges = []) {
            this.edges = deepClone(edges);
            this._clearSelection();
            this._renderAll();
            this._fireChange();
            return this;
        }

        getNodes() {
            return this.getGraph().nodes;
        }

        getEdges() {
            return this.getGraph().edges;
        }

        toObject() {
            return Object.assign(this.getGraph(), { viewport: Object.assign({}, this.viewport) });
        }

        fromObject(flowObject = {}) {
            this.setGraph({ nodes: flowObject.nodes || [], edges: flowObject.edges || [] });
            if (flowObject.viewport) this.setViewport(flowObject.viewport);
            return this;
        }

        getViewport() {
            return Object.assign({}, this.viewport);
        }

        setViewport(viewport = {}) {
            this.viewport.x = typeof viewport.x === "number" ? viewport.x : this.viewport.x;
            this.viewport.y = typeof viewport.y === "number" ? viewport.y : this.viewport.y;
            this.viewport.scale = typeof viewport.scale === "number"
                ? clamp(viewport.scale, ZOOM_MIN, ZOOM_MAX)
                : this.viewport.scale;
            this._applyViewport();
            this._updateBackground();
            this._renderAll();
            return this;
        }

        screenToFlowPosition(point) {
            return this._screenToCanvas(point.x, point.y);
        }

        flowToScreenPosition(point) {
            return this._canvasToScreen(point.x, point.y);
        }

        /**
         * Add a node
         */
        addNode(data = {}) {
            const node = Object.assign(
                {
                    id: "n_" + uid(),
                    label: "Node",
                    position: { x: 100, y: 100 },
                    type: "default",
                },
                data,
            );
            this.nodes.push(node);
            this._renderAll();
            this._fireChange();
            return node;
        }

        /**
         * Remove a node (and its edges)
         */
        removeNode(id) {
            this.nodes = this.nodes.filter(n => n.id !== id);
            this.edges = this.edges.filter(e => e.source !== id && e.target !== id);
            this._selectedNodeIds.delete(id);
            this._renderAll();
            this._fireChange();
            return this;
        }

        /**
         * Update a node's data
         */
        updateNode(id, data) {
            const node = this._getNode(id);
            if (node) Object.assign(node, data);
            this._renderAll();
            this._fireChange();
            return this;
        }

        /**
         * Add an edge
         */
        addEdge(data = {}) {
            const edge = Object.assign({ id: "e_" + uid() }, this._options.defaultEdgeOptions, data);
            this.edges.push(edge);
            this._renderAll();
            this._fireChange();
            return edge;
        }

        /**
         * Remove an edge
         */
        removeEdge(id) {
            this.edges = this.edges.filter(e => e.id !== id);
            this._selectedEdgeIds.delete(id);
            this._renderAll();
            this._fireChange();
            return this;
        }

        /**
         * Select nodes / edges by id
         */
        select({ nodeIds = [], edgeIds = [] } = {}) {
            this._clearSelection();
            nodeIds.forEach(id => this._selectedNodeIds.add(id));
            edgeIds.forEach(id => this._selectedEdgeIds.add(id));
            this._renderAll();
            this._emitSelection();
            return this;
        }

        selectAll() {
            this.nodes.forEach(n => this._selectedNodeIds.add(n.id));
            this.edges.forEach(e => this._selectedEdgeIds.add(e.id));
            this._renderAll();
            this._emitSelection();
            return this;
        }

        /**
         * Fit all nodes in view
         */
        fitView(padding = 60) {
            if (!this.nodes.length) return this;
            let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
            for (const n of this.nodes) {
                minX = Math.min(minX, n.position.x);
                minY = Math.min(minY, n.position.y);
                maxX = Math.max(maxX, n.position.x + (n.width || 180));
                maxY = Math.max(maxY, n.position.y + 60);
            }
            const { w, h } = this._containerSize();
            const scaleX = (w - padding * 2) / (maxX - minX);
            const scaleY = (h - padding * 2) / (maxY - minY);
            const scale = clamp(Math.min(scaleX, scaleY), ZOOM_MIN, ZOOM_MAX);
            const cx = (minX + maxX) / 2;
            const cy = (minY + maxY) / 2;

            this.viewport.scale = scale;
            this.viewport.x = w / 2 - cx * scale;
            this.viewport.y = h / 2 - cy * scale;

            this._applyViewport();
            this._updateBackground();
            this._renderAll();
            return this;
        }

        /**
         * Pan to absolute canvas position
         */
        panTo(x, y, animate = false) {
            if (animate) {
                const startX = this.viewport.x;
                const startY = this.viewport.y;
                const dur = 300;
                const start = performance.now();
                const step = (now) => {
                    const t = Math.min((now - start) / dur, 1);
                    const ease = t < 0.5 ? 2 * t * t : -1 + (4 - 2 * t) * t;
                    this.viewport.x = startX + (x - startX) * ease;
                    this.viewport.y = startY + (y - startY) * ease;
                    this._applyViewport();
                    this._updateBackground();
                    this._renderAll();
                    if (t < 1) requestAnimationFrame(step);
                };
                requestAnimationFrame(step);
            } else {
                this.viewport.x = x;
                this.viewport.y = y;
                this._applyViewport();
                this._updateBackground();
                this._renderAll();
            }
            return this;
        }

        zoomIn(factor = 1.2) {
            const { w, h } = this._containerSize();
            const newScale = clamp(this.viewport.scale * factor, ZOOM_MIN, ZOOM_MAX);
            const f = newScale / this.viewport.scale;
            this.viewport.x = w / 2 - f * (w / 2 - this.viewport.x);
            this.viewport.y = h / 2 - f * (h / 2 - this.viewport.y);
            this.viewport.scale = newScale;
            this._applyViewport();
            this._updateBackground();
            this._renderAll();
            return this;
        }

        zoomOut(factor = 1.2) {
            return this.zoomIn(1 / factor);
        }

        setZoom(scale) {
            const { w, h } = this._containerSize();
            const newScale = clamp(scale, ZOOM_MIN, ZOOM_MAX);
            const f = newScale / this.viewport.scale;
            this.viewport.x = w / 2 - f * (w / 2 - this.viewport.x);
            this.viewport.y = h / 2 - f * (h / 2 - this.viewport.y);
            this.viewport.scale = newScale;
            this._applyViewport();
            this._updateBackground();
            this._renderAll();
            return this;
        }

        toggleLock() {
            this._locked = !this._locked;
            this._controls.setLocked(this._locked);
            return this;
        }

        setTheme(theme) {
            this._applyTheme(theme);
            return this;
        }

        /**
         * Listen to flow events
         */
        on(event, callback) {
            const handler = e => callback(e.detail);
            this._container.addEventListener("fd:" + event, handler);
            return () => this._container.removeEventListener("fd:" + event, handler);
        }

        /**
         * Destroy and clean up
         */
        destroy() {
            if (this._boundEvents) {
                this._root.removeEventListener("wheel", this._boundEvents.wheel);
                this._root.removeEventListener("mousedown", this._boundEvents.mousedown);
                window.removeEventListener("mousemove", this._boundEvents.mousemove);
                window.removeEventListener("mouseup", this._boundEvents.mouseup);
                this._root.removeEventListener("dblclick", this._boundEvents.dblclick);
                document.removeEventListener("keydown", this._boundEvents.keydown);
                document.removeEventListener("keyup", this._boundEvents.keyup);
                this._root.removeEventListener("contextmenu", this._boundEvents.contextmenu);
                this._root.removeEventListener("mouseover", this._boundEvents.mouseover);
                this._root.removeEventListener("mouseout", this._boundEvents.mouseout);
                this._boundEvents = null;
            }
            this._root.remove();
        }
    }

    // ─── CSS ──────────────────────────────────────────────────────────────────

    FlowDesigner.CSS = `
    .fd-container {
      position: relative;
      overflow: hidden;
      width: 100%;
      height: 100%;
      background: var(--fd-bg);
      font-family: 'SF Pro Display', 'Segoe UI', system-ui, sans-serif;
      font-size: 13px;
      color: var(--fd-node-text);
      user-select: none;
      box-sizing: border-box;
    }

    .fd-root {
      position: absolute;
      inset: 0;
      overflow: hidden;
      cursor: default;
    }

    .fd-root--space-panning,
    .fd-root--space-panning .fd-node,
    .fd-root--space-panning .fd-handle {
      cursor: grab !important;
    }

    .fd-viewport {
      position: absolute;
      top: 0; left: 0;
      transform-origin: 0 0;
      will-change: transform;
    }

    .fd-node-layer {
      position: absolute;
      top: 0; left: 0;
    }

    /* ── Node ── */
    .fd-node {
      position: absolute;
      top: 0; left: 0;
      display: flex;
      align-items: stretch;
      background: var(--fd-node-bg);
      border: 1.5px solid var(--fd-node-border);
      border-radius: 10px;
      box-shadow: var(--fd-shadow);
      min-width: ${NODE_MIN_W}px;
      transition: border-color 0.15s, box-shadow 0.15s, opacity 0.15s, translate 0.18s ease;
      cursor: grab;
      z-index: 1;
      will-change: transform, translate;
    }

    .fd-node:active { cursor: grabbing; }

    .fd-node--selected {
      border-color: var(--fd-node-border-selected) !important;
      box-shadow: 0 0 0 2px rgba(99,102,241,0.2), var(--fd-shadow);
      z-index: 10;
    }

    .fd-node--dragging {
      opacity: 0.92;
      box-shadow: 0 16px 48px rgba(0,0,0,0.5);
      z-index: 100;
      cursor: grabbing;
    }

    .fd-node--drop-invalid {
      opacity: 0.36;
      filter: saturate(0.7);
    }

    /* Node types */
    .fd-node--input .fd-node__header { background: linear-gradient(90deg, #22263a, #1e253d); border-bottom-color: rgba(99,102,241,0.3); }
    .fd-node--output .fd-node__header { background: linear-gradient(90deg, #22263a, #1c2639); border-bottom-color: rgba(34,211,238,0.3); }

    .fd-node__inner {
      flex: 1;
      min-width: 0;
    }

    .fd-node__header {
      display: flex;
      align-items: center;
      gap: 7px;
      padding: 9px 13px;
      background: var(--fd-node-header);
      border-bottom: 1px solid var(--fd-node-border);
      border-radius: 8px 8px 0 0;
      font-weight: 600;
      font-size: 12.5px;
      color: var(--fd-node-text);
      letter-spacing: 0.01em;
    }

    .fd-node__icon {
      font-size: 14px;
      line-height: 1;
    }

    .fd-node__title {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .fd-node__badge {
      background: rgba(99,102,241,0.18);
      color: #818cf8;
      border: 1px solid rgba(99,102,241,0.25);
      border-radius: 20px;
      font-size: 10px;
      font-weight: 600;
      padding: 1px 7px;
      letter-spacing: 0.02em;
    }

    .fd-node__content {
      padding: 10px 13px;
      color: var(--fd-node-subtext);
      font-size: 12px;
      line-height: 1.5;
    }

    /* ── Handles ── */
    .fd-node__handles {
      display: flex;
      flex-direction: column;
      justify-content: space-around;
      position: relative;
      width: 0;
    }

    .fd-node__handles--left { order: -1; }
    .fd-node__handles--right { order: 99; }

    .fd-handle {
      position: absolute;
      width: ${HANDLE_RADIUS * 2}px;
      height: ${HANDLE_RADIUS * 2}px;
      border-radius: 50%;
      background: var(--fd-node-bg);
      border: 2px solid var(--fd-handle);
      cursor: crosshair;
      z-index: 10;
      transform: translateY(-50%);
      transition: background 0.15s, border-color 0.15s, transform 0.15s, box-shadow 0.15s;
      box-sizing: border-box;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .fd-handle--left {
      left: -${HANDLE_RADIUS}px;
    }

    .fd-handle--right {
      right: -${HANDLE_RADIUS}px;
    }

    .fd-handle--hover,
    .fd-handle:hover {
      background: var(--fd-handle-hover);
      border-color: var(--fd-handle-hover);
      transform: translateY(-50%) scale(1.32);
      box-shadow: 0 0 0 4px color-mix(in srgb, var(--fd-handle-hover) 22%, transparent), 0 0 18px color-mix(in srgb, var(--fd-handle-hover) 45%, transparent);
    }

    .fd-handle--connectable {
      background: var(--fd-handle-connected) !important;
      border-color: var(--fd-handle-connected) !important;
      transform: translateY(-50%) scale(1.35) !important;
      box-shadow: 0 0 0 5px rgba(34,211,238,0.2) !important;
    }

    .fd-handle__plus {
      color: #fff;
      font-size: 12px;
      font-weight: 700;
      line-height: 1;
      opacity: 0;
      transform: scale(0.55);
      transition: opacity 0.14s ease, transform 0.14s ease;
      pointer-events: none;
    }

    .fd-handle--source:hover .fd-handle__plus,
    .fd-handle--source.fd-handle--hover .fd-handle__plus,
    .fd-handle--source.fd-handle--connectable .fd-handle__plus {
      opacity: 1;
      transform: scale(1);
    }

    .fd-handle[data-handle-branch="true"] {
      border-color: var(--fd-branch-true);
    }

    .fd-handle[data-handle-branch="false"] {
      border-color: var(--fd-branch-false);
    }

    .fd-handle[data-handle-branch="true"]:hover,
    .fd-handle[data-handle-branch="true"].fd-handle--hover {
      background: var(--fd-branch-true);
      border-color: var(--fd-branch-true);
    }

    .fd-handle[data-handle-branch="false"]:hover,
    .fd-handle[data-handle-branch="false"].fd-handle--hover {
      background: var(--fd-branch-false);
      border-color: var(--fd-branch-false);
    }

    /* ── Edges ── */
    .fd-edge path:first-child {
      transition: stroke 0.14s ease, stroke-width 0.14s ease, opacity 0.14s ease;
    }

    .fd-edge--branch-true path:first-child,
    .fd-edge--branch-false path:first-child {
      stroke-dasharray: 0;
    }

    .fd-edge--dimmed path:first-child {
      opacity: 0.22;
    }

    .fd-edge--insert-target path:first-child {
      filter: drop-shadow(0 0 6px color-mix(in srgb, var(--fd-insert) 50%, transparent));
      stroke-dasharray: 8 5;
      animation: fd-dash 0.7s linear infinite;
    }

    .fd-edge-delete {
      opacity: 0;
      transform-box: fill-box;
      transform-origin: center;
      pointer-events: none;
      cursor: pointer;
      transition: opacity 0.12s ease, scale 0.12s ease;
      scale: 0.86;
    }

    .fd-edge:hover .fd-edge-delete,
    .fd-edge:focus-within .fd-edge-delete,
    .fd-edge--selected .fd-edge-delete {
      opacity: 1;
      pointer-events: auto;
      scale: 1;
    }

    .fd-edge-delete circle {
      fill: var(--fd-node-bg);
      stroke: color-mix(in srgb, var(--fd-edge-hover) 72%, transparent);
      stroke-width: 1.5;
      filter: drop-shadow(0 6px 12px rgba(0,0,0,0.18));
    }

    .fd-edge-delete text {
      fill: var(--fd-edge-hover);
      font-family: inherit;
      font-size: 16px;
      font-weight: 700;
    }

    .fd-edge-delete:hover circle,
    .fd-edge-delete:focus circle {
      fill: var(--fd-edge-hover);
      stroke: var(--fd-edge-hover);
    }

    .fd-edge-delete:hover text,
    .fd-edge-delete:focus text {
      fill: #fff;
    }

    .fd-edge__label--true,
    .fd-edge__label--false {
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }

    .fd-edge__label--true { fill: var(--fd-branch-true); }
    .fd-edge__label--false { fill: var(--fd-branch-false); }

    .fd-ghost-edge path {
      filter: drop-shadow(0 0 8px color-mix(in srgb, var(--fd-handle-hover) 45%, transparent));
      animation: fd-dash 0.7s linear infinite;
    }

    .fd-insert-placeholder {
      position: absolute;
      top: 0;
      left: 0;
      width: 116px;
      height: 40px;
      align-items: center;
      justify-content: center;
      gap: 7px;
      border: 1.5px dashed color-mix(in srgb, var(--fd-insert) 78%, transparent);
      border-radius: 8px;
      background: color-mix(in srgb, var(--fd-bg) 72%, transparent);
      color: var(--fd-node-text);
      box-shadow: 0 10px 30px rgba(0,0,0,0.24), 0 0 0 4px color-mix(in srgb, var(--fd-insert) 10%, transparent);
      pointer-events: none;
      z-index: 80;
      will-change: transform, opacity;
      animation: fd-placeholder-pulse 1.2s ease-in-out infinite;
      backdrop-filter: blur(10px);
    }

    .fd-insert-placeholder__icon {
      width: 18px;
      height: 18px;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      border-radius: 50%;
      background: var(--fd-insert);
      color: #fff;
      font-weight: 700;
      line-height: 1;
    }

    @keyframes fd-placeholder-pulse {
      0%, 100% { opacity: 0.82; box-shadow: 0 10px 30px rgba(0,0,0,0.22), 0 0 0 3px color-mix(in srgb, var(--fd-insert) 8%, transparent); }
      50% { opacity: 1; box-shadow: 0 10px 30px rgba(0,0,0,0.26), 0 0 0 7px color-mix(in srgb, var(--fd-insert) 14%, transparent); }
    }

    @keyframes fd-dash {
      to { stroke-dashoffset: -13; }
    }

    /* ── Selection box ── */
    .fd-selection-box {
      position: absolute;
      border: 1.5px solid var(--fd-selection-border);
      background: var(--fd-selection-bg);
      border-radius: 4px;
      pointer-events: none;
      z-index: 50;
    }

    /* ── Controls ── */
    .fd-controls {
      position: absolute;
      bottom: 16px;
      left: 16px;
      display: flex;
      flex-direction: column;
      gap: 2px;
      background: var(--fd-controls-bg);
      border: 1px solid var(--fd-controls-border);
      border-radius: 8px;
      padding: 4px;
      box-shadow: var(--fd-shadow);
      z-index: 100;
    }

    .fd-controls__btn {
      width: 28px;
      height: 28px;
      border-radius: 5px;
      border: none;
      background: transparent;
      color: var(--fd-controls-text);
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: background 0.12s, color 0.12s;
      opacity: 0.75;
    }

    .fd-controls__btn:hover {
      background: rgba(99,102,241,0.12);
      opacity: 1;
    }

    .fd-controls__btn--active {
      background: rgba(99,102,241,0.18);
      color: #818cf8;
      opacity: 1;
    }

    /* ── MiniMap ── */
    .fd-minimap {
      position: absolute;
      bottom: 16px;
      right: 16px;
      border-radius: 8px;
      overflow: hidden;
      border: 1px solid var(--fd-controls-border);
      background: var(--fd-minimap-bg);
      box-shadow: var(--fd-shadow);
      z-index: 100;
      cursor: pointer;
      opacity: 0.85;
      transition: opacity 0.15s;
    }

    .fd-minimap:hover { opacity: 1; }
  `;

    FlowDesigner.THEMES = THEMES;

    return FlowDesigner;
})();

if (typeof window !== "undefined") {
    window.FlowDesigner = FlowDesigner;
}

export default FlowDesigner;
