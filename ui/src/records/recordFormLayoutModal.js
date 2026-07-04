import GridLayout from "@/base/gridLayout";

window.app = window.app || {};
window.app.modals = window.app.modals || {};
window.app.consts = window.app.consts || {};
window.app.consts.FORM_LAYOUT_STORAGE_PREFIX = "pbFormLayout_";

const COLUMNS = 12;

window.app.modals.openRecordFormLayout = function(collection, options = {}) {
    const modal = recordFormLayoutModal(collection, options);
    if (!modal) {
        return;
    }

    document.body.appendChild(modal);
    app.modals.open(modal);
};

function recordFormLayoutModal(collection, options = {}) {
    if (!collection?.id) {
        console.warn("[recordFormLayoutModal] missing required collection");
        return;
    }

    let modal;
    let canvasGrid;
    let trashGrid;

    const initialPreference = readLayoutPreference(collection);
    const local = store({
        layout: normalizeLayout(collection, initialPreference),
        hidden: initialPreference.hidden,
        selectedFieldId: "",
        isDirty: false,
        get availableFields() {
            const hiddenIds = new Set(local.hidden);
            return formFields(collection).filter((field) => hiddenIds.has(field.id));
        },
    });

    function save() {
        const preference = {
            layout: local.layout,
            hidden: local.hidden,
        };

        app.utils.saveLocalHistory(layoutStorageKey(collection), preference);
        document.dispatchEvent(
            new CustomEvent("record:form-layout-change", { detail: { collectionId: collection.id } }),
        );
        options.onsave?.(preference);
        app.modals.close(modal);
        app.toasts.success(`Successfully updated "${collection.name}" form layout.`, { key: "recordFormLayout" });
    }

    function resetLayout() {
        local.hidden = [];
        local.layout = defaultLayout(collection);
        local.selectedFieldId = "";
        local.isDirty = true;
        rerenderCanvas();
    }

    function syncLayout(layout) {
        const hiddenIds = new Set(local.hidden);
        const knownFields = new Map(
            formFields(collection)
                .filter((field) => !hiddenIds.has(field.id))
                .map((field) => [field.id, field]),
        );

        local.layout = layout
            .filter((item) => knownFields.has(item.id))
            .map(normalizeItem)
            .sort(layoutSorter);
        local.isDirty = true;
    }

    function addSelectedField() {
        const field = local.availableFields.find((candidate) => candidate.id == local.selectedFieldId);
        if (!field || !canvasGrid) {
            return;
        }

        local.hidden = local.hidden.filter((id) => id != field.id);
        const nextY = local.layout.reduce((max, item) => Math.max(max, item.y + item.h), 0);
        canvasGrid.addItem(createFieldCard(field), {
            id: field.id,
            x: 0,
            y: nextY,
            w: COLUMNS,
            h: 1,
            minW: 3,
            minH: 1,
        });
        local.selectedFieldId = "";
        syncLayout(canvasGrid.getLayout());
    }

    function removeFieldFromCanvas(id) {
        if (!id || local.hidden.includes(id)) {
            return;
        }

        canvasGrid?.removeItem(id);
        trashGrid?.removeItem(id);
        local.hidden = [...local.hidden, id];
        syncLayout(canvasGrid?.getLayout() || []);
    }

    function rerenderCanvas() {
        const board = modal?.querySelector(".record-form-layout-board");
        if (!board) {
            return;
        }

        canvasGrid?.destroy();
        canvasGrid = null;
        board.replaceChildren(
            ...local.layout.map((item) => {
                const field = collection.fields?.find((candidate) => candidate.id == item.id);
                return field ? createFieldCard(field, item) : null;
            }).filter(Boolean),
        );
        initCanvasGrid(board);
    }

    function initCanvasGrid(board) {
        syncGridCardAttributes(board);

        canvasGrid = new GridLayout(board, {
            columns: COLUMNS,
            rowHeight: 58,
            gap: 10,
            breakpoints: { 0: 4, 600: 8, 900: 12 },
            handleSelector: ".record-form-layout-card-handle",
            group: "record-form-layout-" + collection.id,
        });
        canvasGrid.on("change", syncLayout);
    }

    function createFieldCard(field, item = {}) {
        const layoutItem = {
            id: field.id,
            x: item.x ?? 0,
            y: item.y ?? 0,
            w: item.w ?? COLUMNS,
            h: item.h ?? 1,
        };

        return t.div(
            {
                className: "record-form-layout-card",
                onmount: (el) => setGridCardAttributes(el, layoutItem),
            },
            t.div(
                { className: "record-form-layout-card-head" },
                t.i({
                    className: "ri-drag-move-2-line record-form-layout-card-handle",
                    ariaLabel: "Drag field",
                }),
                t.i({
                    className: app.fieldTypes[field.type]?.icon || app.utils.fallbackFieldIcon,
                    ariaHidden: true,
                }),
                t.span({ className: "txt txt-ellipsis" }, field.name),
                t.span({ className: "label" }, field.type),
            ),
        );
    }

    function syncGridCardAttributes(board) {
        const itemsById = new Map(local.layout.map((item) => [item.id, item]));
        board.querySelectorAll(":scope > .record-form-layout-card").forEach((el, index) => {
            const item = itemsById.get(el.dataset.id) || local.layout[index];
            if (item) {
                setGridCardAttributes(el, item);
            }
        });
    }

    function setGridCardAttributes(el, item) {
        el.dataset.id = item.id;
        el.dataset.x = item.x ?? 0;
        el.dataset.y = item.y ?? 0;
        el.dataset.w = item.w ?? COLUMNS;
        el.dataset.h = item.h ?? 1;
        el.dataset.minW = 3;
        el.dataset.minH = 1;
    }

    modal = t.div(
        {
            className: "modal full record-form-layout-modal",
            onafteropen: (el) => {
                const board = el.querySelector(".record-form-layout-board");
                const trash = el.querySelector(".record-form-layout-trash-board");
                if (!board || !trash || canvasGrid || trashGrid) {
                    return;
                }

                initCanvasGrid(board);
                trashGrid = new GridLayout(trash, {
                    columns: 1,
                    rowHeight: 58,
                    gap: 8,
                    resizable: false,
                    group: "record-form-layout-" + collection.id,
                    breakpoints: { 0: 1 },
                });
                trashGrid.on("itemadded", ({ id }) => removeFieldFromCanvas(id));
            },
            onafterclose: (el) => {
                canvasGrid?.destroy();
                trashGrid?.destroy();
                canvasGrid = null;
                trashGrid = null;
                el.remove();
            },
        },
        t.header(
            { className: "modal-header record-form-layout-header" },
            t.div(
                null,
                t.h5(
                    { className: "modal-title record-form-layout-title" },
                    "Form",
                    t.span(null, "Builder"),
                ),
            ),
            t.div(
                { className: "record-form-layout-meta" },
                t.span(null, "fields: ", t.b(null, () => local.layout.length)),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn secondary transparent circle modal-close-btn",
                    ariaLabel: app.attrs.tooltip("Close"),
                    onclick: () => app.modals.close(modal),
                },
                t.i({ className: "ri-close-line", ariaHidden: true }),
            ),
        ),
        t.div(
            { className: "modal-content" },
            t.div(
                { className: "record-form-layout-shell" },
                t.div(
                    { className: "record-form-layout-main" },
                    t.div(
                        { className: "record-form-layout-toolbar" },
                        app.components.select({
                            className: "record-form-layout-field-select",
                            value: () => local.selectedFieldId,
                            disabled: () => !local.availableFields.length,
                            placeholder: () => local.availableFields.length ? "Add field" : "No fields to add",
                            options: () =>
                                local.availableFields.map((field) => ({
                                    value: field.id,
                                    label: () => t.span(null, field.name, " (", field.type, ")"),
                                })),
                            onchange: (selected) => {
                                local.selectedFieldId = selected?.[0]?.value || "";
                            },
                        }),
                        t.button(
                            {
                                type: "button",
                                className: "btn outline",
                                disabled: () => !local.selectedFieldId,
                                onclick: () => addSelectedField(),
                            },
                            t.i({ className: "ri-add-line", ariaHidden: true }),
                            t.span({ className: "txt" }, "Add field"),
                        ),
                    ),
                    t.p({ className: "record-form-layout-panel-title" }, "Form canvas"),
                    t.div(
                        { className: "record-form-layout-board" },
                        local.layout.map((item) => {
                            const field = collection.fields?.find((candidate) => candidate.id == item.id);
                            return field ? createFieldCard(field, item) : null;
                        }),
                    ),
                ),
                t.aside(
                    { className: "record-form-layout-trash" },
                    t.p({ className: "record-form-layout-panel-title" }, "Trash"),
                    t.div(
                        { className: "record-form-layout-trash-board" },
                        t.div(
                            { className: "record-form-layout-trash-hint" },
                            t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                            t.span({ className: "txt" }, "Trash"),
                        ),
                    ),
                    t.p({ className: "record-form-layout-panel-title" }, "Available fields"),
                    t.div(
                        { className: "record-form-layout-hidden-list" },
                        () => {
                            if (!local.availableFields.length) {
                                return t.div({ className: "txt-hint" }, "All fields are on the form.");
                            }

                            return local.availableFields.map((field) =>
                                t.button(
                                    {
                                        type: "button",
                                        className: "btn sm outline",
                                        onclick: () => {
                                            local.selectedFieldId = field.id;
                                            addSelectedField();
                                        },
                                    },
                                    t.i({
                                        className: app.fieldTypes[field.type]?.icon || app.utils.fallbackFieldIcon,
                                        ariaHidden: true,
                                    }),
                                    t.span({ className: "txt txt-ellipsis" }, field.name),
                                )
                            );
                        },
                    ),
                ),
            ),
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    onclick: () => resetLayout(),
                },
                t.i({ className: "ri-restart-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Reset"),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn transparent",
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Cancel"),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn",
                    disabled: () => !local.isDirty,
                    onclick: () => save(),
                },
                t.span({ className: "txt" }, "Save layout"),
            ),
        ),
    );

    return modal;
}

export function layoutStorageKey(collection) {
    return app.consts.FORM_LAYOUT_STORAGE_PREFIX + collection.id;
}

export function readLayoutPreference(collection) {
    return normalizePreference(app.utils.getLocalHistory(layoutStorageKey(collection), null));
}

function normalizePreference(raw) {
    if (Array.isArray(raw)) {
        return { layout: raw, hidden: [] };
    }

    return {
        layout: Array.isArray(raw?.layout) ? raw.layout : [],
        hidden: Array.isArray(raw?.hidden) ? raw.hidden : [],
    };
}

export function formFields(collection, excludedFields = []) {
    return (collection?.fields || []).filter((field) => {
        return app.fieldTypes[field.type]?.input
            && !field.primaryKey
            && !excludedFields.includes(field.name);
    });
}

export function defaultLayout(collection, excludedFields = []) {
    let y = 0;

    return formFields(collection, excludedFields).map((field) => ({
        id: field.id,
        x: 0,
        y: y++,
        w: COLUMNS,
        h: 1,
    }));
}

export function normalizeLayout(collection, preference = {}, excludedFields = []) {
    const normalizedPreference = normalizePreference(preference);
    const hiddenIds = new Set(normalizedPreference.hidden);
    const fields = formFields(collection, excludedFields).filter((field) => !hiddenIds.has(field.id));
    const fieldIds = new Set(fields.map((field) => field.id));
    const used = new Set();
    const normalized = [];

    for (const item of normalizedPreference.layout || []) {
        if (!fieldIds.has(item?.id) || used.has(item.id)) {
            continue;
        }

        used.add(item.id);
        normalized.push(normalizeItem(item));
    }

    let nextY = normalized.reduce((max, item) => Math.max(max, item.y + item.h), 0);
    for (const field of fields) {
        if (used.has(field.id)) {
            continue;
        }

        normalized.push({
            id: field.id,
            x: 0,
            y: nextY++,
            w: COLUMNS,
            h: 1,
        });
    }

    return normalized.sort(layoutSorter);
}

function normalizeItem(item) {
    return {
        id: item.id,
        x: Math.max(0, Math.min(item.x || 0, COLUMNS - 1)),
        y: Math.max(0, item.y || 0),
        w: Math.max(1, Math.min(item.w || COLUMNS, COLUMNS)),
        h: Math.max(1, item.h || 1),
    };
}

export function layoutSorter(a, b) {
    return (a.y - b.y) || (a.x - b.x);
}
