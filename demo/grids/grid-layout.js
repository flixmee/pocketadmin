/**
 * grid-layout.js
 * A dependency-free, vanilla JS draggable + resizable + responsive grid layout.
 *
 * Usage (declarative):
 *   <div id="board">
 *     <div data-x="0" data-y="0" data-w="4" data-h="2">A</div>
 *     <div data-x="4" data-y="0" data-w="4" data-h="2" data-static="true">B (locked)</div>
 *   </div>
 *   <script type="module">
 *     import GridLayout from './grid-layout.js';
 *     const grid = new GridLayout('#board', { columns: 12, rowHeight: 80, gap: 12 });
 *     grid.on('change', layout => console.log(layout));
 *   </script>
 *
 * Usage (programmatic):
 *   grid.addItem('<strong>New card</strong>', { x: 0, y: 0, w: 3, h: 2 });
 *
 * Cross-area drag & drop: give two GridLayout instances the same `group`
 * name and items can be dragged from one into the other.
 *
 * Restricting drag to a handle (useful when items contain editable text,
 * inputs, or buttons): pass `handleSelector: '.drag-handle'` and add an
 * element with that class inside each item.
 *
 * No external dependencies. Uses the Pointer Events API so mouse, touch and
 * pen all work through a single code path.
 */

const DEFAULTS = {
  columns: 12,                 // columns at the widest breakpoint
  rowHeight: 80,                // px height of one grid row
  gap: 12,                      // px gap between cells (horizontal + vertical)
  draggable: true,              // allow dragging items
  resizable: true,              // allow resizing items
  handleSelector: null,         // if set, only pointerdown on a matching descendant starts a drag
                                 // (e.g. '.drag-handle') — leaves inputs/labels/buttons free to be clicked
  autoCompact: true,            // remove vertical gaps after every commit
  itemSelector: '[data-x]',     // selector used to discover pre-existing DOM items
  group: 'default',             // grids sharing a group name can exchange items by dragging
  // Responsive breakpoints: { minWidthPx: columnCount }. The widest matching
  // entry (container width >= key) wins. Keys need not match `columns`;
  // 0 should always be present as the smallest fallback.
  breakpoints: { 0: 1, 480: 2, 768: 4, 1024: 6, 1280: 12 },
};

// Grids sharing the same `group` name can hand items off to one another
// while dragging. Keyed by group name -> Set of live GridLayout instances.
const groupRegistry = new Map();

let styleInjected = false;

function injectStyles() {
  if (styleInjected) return;
  styleInjected = true;
  const style = document.createElement('style');
  style.setAttribute('data-grid-layout', '');
  style.textContent = `
.gl-container{position:relative;box-sizing:border-box;width:100%;transition:outline-color .12s ease;}
.gl-container.gl-drop-target{outline:2px dashed var(--gl-dropzone-color,rgba(94,234,212,.7));outline-offset:-2px;}
.gl-item{position:absolute;box-sizing:border-box;overflow:hidden;
  background:var(--gl-item-bg,#fff);
  border:1px solid var(--gl-item-border,rgba(0,0,0,.12));
  border-radius:var(--gl-item-radius,8px);
  transition:left .18s ease,top .18s ease,width .18s ease,height .18s ease;}
.gl-item.gl-dragging,.gl-item.gl-resizing{transition:none !important;z-index:1000;user-select:none;}
.gl-item.gl-static{outline:1px dashed transparent;}
.gl-resize-handle{position:absolute;right:2px;bottom:2px;width:14px;height:14px;
  cursor:se-resize;touch-action:none;
  border-right:2px solid var(--gl-handle-color,rgba(0,0,0,.35));
  border-bottom:2px solid var(--gl-handle-color,rgba(0,0,0,.35));
  border-radius:0 0 3px 0;opacity:.6;}
.gl-resize-handle:hover{opacity:1;}
.gl-placeholder{position:absolute;z-index:0;pointer-events:none;border-radius:6px;
  background:var(--gl-placeholder-bg,rgba(59,130,246,.12));
  border:2px dashed var(--gl-placeholder-border,rgba(59,130,246,.55));
  transition:left .12s ease,top .12s ease,width .12s ease,height .12s ease;}
`;
  document.head.appendChild(style);
}

const clamp = (v, min, max) => Math.min(Math.max(v, min), max);
let uidCounter = 0;
const uid = () => `gl-${Date.now().toString(36)}-${(uidCounter++).toString(36)}`;

function collides(a, b) {
  return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
}

export default class GridLayout {
  constructor(container, options = {}) {
    this.container = typeof container === 'string' ? document.querySelector(container) : container;
    if (!this.container) throw new Error('GridLayout: container not found');

    this.options = { ...DEFAULTS, ...options, breakpoints: { ...DEFAULTS.breakpoints, ...(options.breakpoints || {}) } };
    this.items = [];          // internal item records: {id,x,y,w,h,minW,minH,maxW,maxH,static,el}
    this._listeners = {};     // event name -> [callbacks]
    this._columns = this.options.columns;
    this._colWidth = 0;
    this._rowHeight = this.options.rowHeight;
    this._gap = this.options.gap;
    this._activeId = null;

    injectStyles();
    this.container.classList.add('gl-container');

    if (!groupRegistry.has(this.options.group)) groupRegistry.set(this.options.group, new Set());
    groupRegistry.get(this.options.group).add(this);

    this._scanExistingItems();
    this._resizeObserver = new ResizeObserver(() => this._onContainerResize());
    this._resizeObserver.observe(this.container);
    this._onContainerResize(true);
  }

  // ---------------------------------------------------------------------
  // Public API
  // ---------------------------------------------------------------------

  /** Add a new item. `content` is an HTML string or an Element. */
  addItem(content, config = {}) {
    const el = content instanceof Element ? content : document.createElement('div');
    if (!(content instanceof Element)) el.innerHTML = content;
    this.container.appendChild(el);

    const item = this._registerItem(el, config);
    this._resolveCollisions(item.id);
    if (this.options.autoCompact) this.compact();
    this._render();
    this._emit('change', this.getLayout());
    return item.id;
  }

  removeItem(id) {
    const idx = this.items.findIndex(i => i.id === id);
    if (idx === -1) return;
    this.items[idx].el.remove();
    this.items.splice(idx, 1);
    if (this.options.autoCompact) this.compact();
    this._render();
    this._emit('change', this.getLayout());
  }

  updateItem(id, patch = {}) {
    const item = this.items.find(i => i.id === id);
    if (!item) return;
    Object.assign(item, patch);
    item.w = clamp(item.w, item.minW, Math.min(item.maxW, this._columns));
    item.h = clamp(item.h, item.minH, item.maxH);
    item.x = clamp(item.x, 0, this._columns - item.w);
    this._resolveCollisions(item.id);
    if (this.options.autoCompact) this.compact();
    this._render();
    this._emit('change', this.getLayout());
  }

  getLayout() {
    return this.items.map(({ id, x, y, w, h, static: s }) => ({ id, x, y, w, h, static: !!s }));
  }

  /** Apply a previously saved layout (matched by id). Unmatched items keep their position. */
  setLayout(layout = []) {
    for (const entry of layout) {
      const item = this.items.find(i => i.id === entry.id);
      if (item) Object.assign(item, entry);
    }
    if (this.options.autoCompact) this.compact();
    this._render();
    this._emit('change', this.getLayout());
  }

  /** Push every non-static item up as far as it can go without overlapping. */
  compact() {
    const sorted = [...this.items].sort((a, b) => a.y - b.y || a.x - b.x);
    for (const item of sorted) {
      if (item.static) continue;
      let targetY = 0;
      for (const other of sorted) {
        if (other === item) continue;
        const overlapsX = other.x < item.x + item.w && other.x + other.w > item.x;
        if (!overlapsX) continue;
        // only items already settled above this one act as a floor
        if (other.y < item.y || other.static) {
          targetY = Math.max(targetY, other.y + other.h);
        }
      }
      item.y = targetY;
    }
  }

  /** Move an item from this grid to another grid instance in the same group. */
  moveItemTo(id, targetGrid, { x = 0, y = 0 } = {}) {
    const item = this.items.find(i => i.id === id);
    if (!item || targetGrid === this) return;
    this._transferItem(item, targetGrid, x, y);
  }

  on(event, cb) {
    (this._listeners[event] = this._listeners[event] || []).push(cb);
    return () => this.off(event, cb);
  }

  off(event, cb) {
    if (!this._listeners[event]) return;
    this._listeners[event] = this._listeners[event].filter(fn => fn !== cb);
  }

  destroy() {
    this._resizeObserver.disconnect();
    groupRegistry.get(this.options.group)?.delete(this);
    for (const item of this.items) {
      item.el.classList.remove('gl-item', 'gl-dragging', 'gl-resizing', 'gl-static');
      item.el.style.position = item.el.style.left = item.el.style.top = '';
      item.el.style.width = item.el.style.height = '';
      const handle = item.el.querySelector(':scope > .gl-resize-handle');
      if (handle) handle.remove();
    }
    this.container.classList.remove('gl-container');
    this.items = [];
    this._listeners = {};
  }

  // ---------------------------------------------------------------------
  // Internal: setup
  // ---------------------------------------------------------------------

  _scanExistingItems() {
    const els = this.container.querySelectorAll(this.options.itemSelector);
    els.forEach(el => this._registerItem(el, {
      x: +el.dataset.x || 0,
      y: +el.dataset.y || 0,
      w: +el.dataset.w || 1,
      h: +el.dataset.h || 1,
      minW: +el.dataset.minW || 1,
      minH: +el.dataset.minH || 1,
      maxW: +el.dataset.maxW || Infinity,
      maxH: +el.dataset.maxH || Infinity,
      static: el.dataset.static === 'true',
    }));
  }

  _registerItem(el, config) {
    const item = {
      id: config.id || el.dataset.id || uid(),
      x: config.x ?? 0,
      y: config.y ?? 0,
      w: config.w ?? 1,
      h: config.h ?? 1,
      minW: config.minW ?? 1,
      minH: config.minH ?? 1,
      maxW: config.maxW ?? Infinity,
      maxH: config.maxH ?? Infinity,
      static: !!config.static,
      el,
    };
    el.classList.add('gl-item');
    el.classList.toggle('gl-static', item.static);
    el.style.position = 'absolute';

    this._bindItemInteractions(item);
    this.items.push(item);
    return item;
  }

  /** (Re)attach the drag/resize listeners for an item, scoped to this grid.
   *  Called on first registration and again after a cross-grid transfer. */
  _bindItemInteractions(item) {
    const el = item.el;
    const oldHandle = el.querySelector(':scope > .gl-resize-handle');
    if (oldHandle) oldHandle.remove();
    if (item._dragHandler) el.removeEventListener('pointerdown', item._dragHandler);

    if (this.options.resizable && !item.static) {
      const handle = document.createElement('div');
      handle.className = 'gl-resize-handle';
      el.appendChild(handle);
      handle.addEventListener('pointerdown', e => {
        e.stopPropagation();
        this._onResizeStart(e, item);
      });
    }
    if (this.options.draggable && !item.static) {
      item._dragHandler = e => {
        if (e.target.closest('.gl-resize-handle')) return;
        if (this.options.handleSelector && !e.target.closest(this.options.handleSelector)) return;
        this._onDragStart(e, item);
      };
      el.addEventListener('pointerdown', item._dragHandler);
    }
  }

  // ---------------------------------------------------------------------
  // Internal: responsive column handling
  // ---------------------------------------------------------------------

  _columnsForWidth(width) {
    const keys = Object.keys(this.options.breakpoints).map(Number).sort((a, b) => a - b);
    let cols = this.options.breakpoints[keys[0]];
    for (const k of keys) if (width >= k) cols = this.options.breakpoints[k];
    return cols;
  }

  _onContainerResize(force = false) {
    const width = this.container.clientWidth;
    const newColumns = this._columnsForWidth(width);
    const columnsChanged = newColumns !== this._columns;
    this._columns = newColumns;
    this._colWidth = (width - this._gap * (this._columns - 1)) / this._columns;

    if (columnsChanged) {
      for (const item of this.items) {
        item.w = Math.min(item.w, this._columns);
        item.x = clamp(item.x, 0, this._columns - item.w);
      }
      if (this.options.autoCompact) this.compact();
    }
    if (columnsChanged || force) this._render();
  }

  // ---------------------------------------------------------------------
  // Internal: cross-grid transfer
  // ---------------------------------------------------------------------

  /** Find the grid (in this grid's group) whose container is under a client point. */
  _findGridAt(clientX, clientY) {
    const peers = groupRegistry.get(this.options.group);
    if (!peers) return null;
    for (const grid of peers) {
      const rect = grid.container.getBoundingClientRect();
      if (clientX >= rect.left && clientX <= rect.right && clientY >= rect.top && clientY <= rect.bottom) {
        return grid;
      }
    }
    return null;
  }

  /** Hand an item off from this grid to `toGrid`, placing it at grid cell (x, y). */
  _transferItem(item, toGrid, x, y) {
    this.items = this.items.filter(i => i !== item);
    toGrid.container.appendChild(item.el);
    item.w = Math.min(item.w, toGrid._columns);
    item.x = clamp(x, 0, toGrid._columns - item.w);
    item.y = Math.max(0, y);
    toGrid.items.push(item);
    toGrid._bindItemInteractions(item);

    toGrid._resolveCollisions(item.id);
    if (toGrid.options.autoCompact) toGrid.compact();
    this._activeId = null;
    toGrid._activeId = null;

    this._render();
    toGrid._render();
    this._emit('itemremoved', { id: item.id, to: toGrid });
    toGrid._emit('itemadded', { id: item.id, from: this });
    this._emit('change', this.getLayout());
    toGrid._emit('change', toGrid.getLayout());
  }

  // ---------------------------------------------------------------------
  // Internal: collisions
  // ---------------------------------------------------------------------

  _resolveCollisions(movedId) {
    let guard = 0;
    let changed = true;
    while (changed && guard < 500) {
      changed = false;
      guard++;
      const sorted = [...this.items].sort((a, b) => a.y - b.y || a.x - b.x);
      for (const a of sorted) {
        for (const b of sorted) {
          if (a === b || b.static) continue;
          const aHasPriority = a.id === movedId || a.static || a.y < b.y || (a.y === b.y && a.x <= b.x);
          if (aHasPriority && collides(a, b)) {
            const newY = a.y + a.h;
            if (b.y < newY) { b.y = newY; changed = true; }
          }
        }
      }
    }
  }

  // ---------------------------------------------------------------------
  // Internal: rendering
  // ---------------------------------------------------------------------

  _cellToPixels(x, y, w, h) {
    return {
      left: x * (this._colWidth + this._gap),
      top: y * (this._rowHeight + this._gap),
      width: w * this._colWidth + (w - 1) * this._gap,
      height: h * this._rowHeight + (h - 1) * this._gap,
    };
  }

  _render() {
    let maxY = 0;
    for (const item of this.items) {
      if (item.id === this._activeId) continue; // being interactively dragged/resized
      const { left, top, width, height } = this._cellToPixels(item.x, item.y, item.w, item.h);
      item.el.style.left = `${left}px`;
      item.el.style.top = `${top}px`;
      item.el.style.width = `${width}px`;
      item.el.style.height = `${height}px`;
      maxY = Math.max(maxY, item.y + item.h);
    }
    this.container.style.height = `${Math.max(0, maxY * (this._rowHeight + this._gap) - this._gap)}px`;
  }

  _createPlaceholder(item) {
    const el = document.createElement('div');
    el.className = 'gl-placeholder';
    this.container.appendChild(el);
    this._updatePlaceholder(el, item.x, item.y, item.w, item.h);
    return el;
  }

  _updatePlaceholder(el, x, y, w, h) {
    const { left, top, width, height } = this._cellToPixels(x, y, w, h);
    el.style.left = `${left}px`;
    el.style.top = `${top}px`;
    el.style.width = `${width}px`;
    el.style.height = `${height}px`;
  }

  _emit(event, payload) {
    (this._listeners[event] || []).forEach(cb => cb(payload));
  }

  // ---------------------------------------------------------------------
  // Internal: drag
  // ---------------------------------------------------------------------

  _onDragStart(e, item) {
    e.preventDefault();
    this._activeId = item.id;
    item.el.classList.add('gl-dragging');
    item.el.setPointerCapture(e.pointerId);

    // Offset between the pointer and the item's top-left corner, in client
    // (viewport) coordinates. Using client space (rather than a delta from
    // the source grid's local coordinates) lets us keep tracking the same
    // offset even if the pointer wanders over a different grid.
    const startRect = item.el.getBoundingClientRect();
    const grabOffsetX = e.clientX - startRect.left;
    const grabOffsetY = e.clientY - startRect.top;

    let placeholder = this._createPlaceholder(item);
    let hoverGrid = this;
    this._emit('dragstart', { id: item.id });

    const placeItemVisually = (clientX, clientY) => {
      // The dragged element stays a child of the *source* container for the
      // whole gesture (simplest option), but its left/top are computed from
      // the live client position so it still visually tracks the cursor
      // even while hovering over a neighboring grid.
      const sourceRect = this.container.getBoundingClientRect();
      item.el.style.left = `${clientX - grabOffsetX - sourceRect.left}px`;
      item.el.style.top = `${clientY - grabOffsetY - sourceRect.top}px`;
    };

    const onMove = ev => {
      placeItemVisually(ev.clientX, ev.clientY);

      const targetGrid = this._findGridAt(ev.clientX, ev.clientY) || this;
      if (targetGrid !== hoverGrid) {
        if (hoverGrid !== this) hoverGrid.container.classList.remove('gl-drop-target');
        if (targetGrid !== this) targetGrid.container.classList.add('gl-drop-target');
        placeholder.remove();
        placeholder = document.createElement('div');
        placeholder.className = 'gl-placeholder';
        targetGrid.container.appendChild(placeholder);
        hoverGrid = targetGrid;
      }

      const targetRect = targetGrid.container.getBoundingClientRect();
      const localX = ev.clientX - grabOffsetX - targetRect.left;
      const localY = ev.clientY - grabOffsetY - targetRect.top;
      const previewW = Math.min(item.w, targetGrid._columns);
      const cellX = clamp(Math.round(localX / (targetGrid._colWidth + targetGrid._gap)), 0, targetGrid._columns - previewW);
      const cellY = Math.max(0, Math.round(localY / (targetGrid._rowHeight + targetGrid._gap)));
      targetGrid._updatePlaceholder(placeholder, cellX, cellY, previewW, item.h);
    };

    const onUp = ev => {
      document.removeEventListener('pointermove', onMove);
      document.removeEventListener('pointerup', onUp);
      item.el.classList.remove('gl-dragging');
      item.el.releasePointerCapture(e.pointerId);
      placeholder.remove();
      hoverGrid.container.classList.remove('gl-drop-target');

      const targetGrid = this._findGridAt(ev.clientX, ev.clientY) || this;
      const targetRect = targetGrid.container.getBoundingClientRect();
      const localX = ev.clientX - grabOffsetX - targetRect.left;
      const localY = ev.clientY - grabOffsetY - targetRect.top;
      const finalW = Math.min(item.w, targetGrid._columns);
      const cellX = clamp(Math.round(localX / (targetGrid._colWidth + targetGrid._gap)), 0, targetGrid._columns - finalW);
      const cellY = Math.max(0, Math.round(localY / (targetGrid._rowHeight + targetGrid._gap)));

      if (targetGrid !== this) {
        this._transferItem(item, targetGrid, cellX, cellY);
        this._emit('dragstop', { id: item.id, movedTo: targetGrid });
        return;
      }

      item.x = cellX;
      item.y = cellY;
      this._resolveCollisions(item.id);
      if (this.options.autoCompact) this.compact();
      this._activeId = null;
      this._render();
      this._emit('dragstop', { id: item.id });
      this._emit('change', this.getLayout());
    };

    document.addEventListener('pointermove', onMove);
    document.addEventListener('pointerup', onUp);
  }

  // ---------------------------------------------------------------------
  // Internal: resize
  // ---------------------------------------------------------------------

  _onResizeStart(e, item) {
    e.preventDefault();
    this._activeId = item.id;
    item.el.classList.add('gl-resizing');
    item.el.setPointerCapture(e.pointerId);

    const startX = e.clientX;
    const startY = e.clientY;
    const startW = item.w * this._colWidth + (item.w - 1) * this._gap;
    const startH = item.h * this._rowHeight + (item.h - 1) * this._gap;
    const placeholder = this._createPlaceholder(item);
    this._emit('resizestart', { id: item.id });

    const onMove = ev => {
      const dx = ev.clientX - startX;
      const dy = ev.clientY - startY;
      const pxW = Math.max(this._colWidth, startW + dx);
      const pxH = Math.max(this._rowHeight, startH + dy);
      item.el.style.width = `${pxW}px`;
      item.el.style.height = `${pxH}px`;

      const targetW = clamp(Math.round((pxW + this._gap) / (this._colWidth + this._gap)), item.minW, Math.min(item.maxW, this._columns - item.x));
      const targetH = clamp(Math.round((pxH + this._gap) / (this._rowHeight + this._gap)), item.minH, item.maxH);
      this._updatePlaceholder(placeholder, item.x, item.y, targetW, targetH);
    };

    const onUp = ev => {
      document.removeEventListener('pointermove', onMove);
      document.removeEventListener('pointerup', onUp);
      item.el.classList.remove('gl-resizing');
      item.el.releasePointerCapture(e.pointerId);
      placeholder.remove();

      const dx = ev.clientX - startX;
      const dy = ev.clientY - startY;
      const pxW = Math.max(this._colWidth, startW + dx);
      const pxH = Math.max(this._rowHeight, startH + dy);
      item.w = clamp(Math.round((pxW + this._gap) / (this._colWidth + this._gap)), item.minW, Math.min(item.maxW, this._columns - item.x));
      item.h = clamp(Math.round((pxH + this._gap) / (this._rowHeight + this._gap)), item.minH, item.maxH);

      this._resolveCollisions(item.id);
      if (this.options.autoCompact) this.compact();
      this._activeId = null;
      this._render();
      this._emit('resizestop', { id: item.id });
      this._emit('change', this.getLayout());
    };

    document.addEventListener('pointermove', onMove);
    document.addEventListener('pointerup', onUp);
  }
}
