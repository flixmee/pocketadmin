import GridLayout from "@/base/gridLayout";

window.app = window.app || {};
window.app.modals = window.app.modals || {};
window.app.consts = window.app.consts || {};
window.app.consts.FORM_LAYOUT_STORAGE_PREFIX = "pbFormLayout_";

const COLUMNS = 12;
const DEFAULT_SECTION_ID = "main";

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
    let sectionGrids = new Map();
    let trashGrid;

    const initialPreference = readLayoutPreference(collection);
    const initialSections = normalizeSections(collection, initialPreference);
    const local = store({
        sections: initialSections,
        hidden: initialPreference.hidden,
        selectedFieldId: "",
        selectedSectionId: initialSections[0]?.id || DEFAULT_SECTION_ID,
        isDirty: false,
        isSaving: false,
        get availableFields() {
            const hiddenIds = new Set(local.hidden);
            return formFields(collection).filter((field) => hiddenIds.has(field.id));
        },
    });

    async function save() {
        if (local.isSaving) {
            return;
        }

        syncSectionsFromGrids(false);

        const preference = {
            layout: flattenSections(local.sections),
            sections: serializeSections(local.sections),
            hidden: local.hidden,
        };

        local.isSaving = true;

        try {
            const updatedCollection = await app.pb.collections.update(collection.id, {
                rearrange: preference,
            });

            app.utils.saveLocalHistory(layoutStorageKey(collection), null);
            Object.assign(collection, structuredClone(updatedCollection));
            app.store.addOrUpdateCollection(structuredClone(updatedCollection));

            document.dispatchEvent(
                new CustomEvent("record:form-layout-change", { detail: { collectionId: collection.id } }),
            );
            options.onsave?.(preference, updatedCollection);
            app.modals.close(modal);
            app.toasts.success(`Successfully updated "${collection.name}" form layout.`, { key: "recordFormLayout" });
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err, false);
                app.toasts.error(err.message || "Failed to save form layout.", { key: "recordFormLayout" });
            }
        }

        local.isSaving = false;
    }

    function resetLayout() {
        local.hidden = [];
        local.sections = defaultSections(collection);
        local.selectedSectionId = local.sections[0]?.id || DEFAULT_SECTION_ID;
        local.selectedFieldId = "";
        local.isDirty = true;
        rerenderCanvas();
    }

    function syncSectionsFromGrids(markDirty = true) {
        const hiddenIds = new Set(local.hidden);
        const knownFields = new Map(
            formFields(collection)
                .filter((field) => !hiddenIds.has(field.id))
                .map((field) => [field.id, field]),
        );
        const used = new Set();

        local.sections = local.sections.map((section) => {
            const grid = sectionGrids.get(section.id);
            const layout = (grid?.getLayout() || section.layout || [])
                .filter((item) => {
                    if (!knownFields.has(item.id) || used.has(item.id)) {
                        return false;
                    }

                    used.add(item.id);
                    return true;
                })
                .map(normalizeItem)
                .sort(layoutSorter);

            section.layout = layout;
            return section;
        });
        if (markDirty) {
            local.isDirty = true;
        }
    }

    function addSelectedField() {
        const field = local.availableFields.find((candidate) => candidate.id == local.selectedFieldId);
        const section = getSelectedSection();
        const grid = sectionGrids.get(section?.id);
        if (!field || !section) {
            return;
        }

        local.hidden = local.hidden.filter((id) => id != field.id);
        const layout = grid?.getLayout() || section.layout || [];
        const nextY = layout.reduce((max, item) => Math.max(max, item.y + item.h), 0);
        const item = {
            id: field.id,
            x: 0,
            y: nextY,
            w: COLUMNS,
            h: 1,
            minW: 3,
            minH: 1,
        };

        if (grid) {
            grid.addItem(createFieldCard(field), item);
        } else {
            section.layout = [...(section.layout || []), item];
            rerenderCanvas();
        }

        local.selectedFieldId = "";
        syncSectionsFromGrids();
    }

    function removeFieldFromCanvas(id) {
        if (!id || local.hidden.includes(id)) {
            return;
        }

        sectionGrids.forEach((grid) => grid.removeItem(id));
        trashGrid?.removeItem(id);
        local.hidden = [...local.hidden, id];
        syncSectionsFromGrids();
    }

    function addSection() {
        syncSectionsFromGrids(false);

        const section = {
            id: createSectionId(),
            name: `Section ${local.sections.length + 1}`,
            description: "",
            layout: [],
        };

        local.sections = [...local.sections, section];
        local.selectedSectionId = section.id;
        local.isDirty = true;
        rerenderCanvas();
    }

    function removeSection(sectionId) {
        syncSectionsFromGrids(false);

        const index = local.sections.findIndex((section) => section.id == sectionId);
        if (index < 0 || local.sections.length <= 1) {
            return;
        }

        const sections = local.sections.map((section) => ({
            ...section,
            layout: [...(section.layout || [])],
        }));
        const [removed] = sections.splice(index, 1);
        const targetIndex = Math.max(0, index - 1);
        const target = sections[targetIndex];
        const nextY = target.layout.reduce((max, item) => Math.max(max, item.y + item.h), 0);

        target.layout.push(
            ...removed.layout.map((item, itemIndex) => ({
                ...normalizeItem(item),
                x: 0,
                y: nextY + itemIndex,
                w: item.w || COLUMNS,
            })),
        );

        local.sections = sections;
        local.selectedSectionId = target.id;
        local.isDirty = true;
        rerenderCanvas();
    }

    function moveSection(sectionId, direction) {
        syncSectionsFromGrids(false);

        const index = local.sections.findIndex((section) => section.id == sectionId);
        const targetIndex = index + direction;
        if (index < 0 || targetIndex < 0 || targetIndex >= local.sections.length) {
            return;
        }

        const sections = [...local.sections];
        [sections[index], sections[targetIndex]] = [sections[targetIndex], sections[index]];
        local.sections = sections;
        local.isDirty = true;
        rerenderCanvas();
    }

    function getSelectedSection() {
        return local.sections.find((section) => section.id == local.selectedSectionId) || local.sections[0];
    }

    function rerenderCanvas() {
        const sectionsEl = modal?.querySelector(".record-form-layout-sections");
        if (!sectionsEl) {
            return;
        }

        destroySectionGrids();
        sectionsEl.replaceChildren(
            ...local.sections.map((section, index) => createSectionPanel(section, index)).filter(Boolean),
        );
        initSectionGrids(sectionsEl);
    }

    function initSectionGrids(root) {
        root.querySelectorAll(".record-form-layout-board").forEach((board) => {
            syncGridCardAttributes(board);

            const grid = new GridLayout(board, {
                columns: COLUMNS,
                rowHeight: 58,
                gap: 10,
                breakpoints: { 0: 4, 600: 8, 900: 12 },
                handleSelector: ".record-form-layout-card-handle",
                group: "record-form-layout-" + collection.id,
            });
            grid.on("change", () => syncSectionsFromGrids());
            sectionGrids.set(board.dataset.sectionId, grid);
        });
    }

    function destroySectionGrids() {
        sectionGrids.forEach((grid) => grid.destroy());
        sectionGrids = new Map();
    }

    function createSectionPanel(section, index) {
        return t.section(
            { className: "record-form-layout-section" },
            t.div(
                { className: "record-form-layout-section-header" },
                t.button(
                    {
                        type: "button",
                        className: () =>
                            `btn sm circle ${
                                local.selectedSectionId == section.id ? "" : "outline"
                            } record-form-layout-section-target`,
                        ariaLabel: app.attrs.tooltip("Add available fields here"),
                        onclick: () => local.selectedSectionId = section.id,
                    },
                    t.i({ className: "ri-add-circle-line", ariaHidden: true }),
                ),
                t.div(
                    { className: "record-form-layout-section-fields" },
                    t.input({
                        type: "text",
                        value: () => section.name,
                        placeholder: "Section name",
                        oninput: (e) => {
                            section.name = e.target.value;
                            local.isDirty = true;
                        },
                    }),
                    t.textarea({
                        value: () => section.description,
                        placeholder: "Description",
                        rows: 1,
                        oninput: (e) => {
                            section.description = e.target.value;
                            local.isDirty = true;
                        },
                    }),
                ),
                t.div(
                    { className: "record-form-layout-section-actions" },
                    t.button(
                        {
                            type: "button",
                            className: "btn sm outline circle",
                            disabled: () => index <= 0,
                            ariaLabel: app.attrs.tooltip("Move section up"),
                            onclick: () => moveSection(section.id, -1),
                        },
                        t.i({ className: "ri-arrow-up-line", ariaHidden: true }),
                    ),
                    t.button(
                        {
                            type: "button",
                            className: "btn sm outline circle",
                            disabled: () => index >= local.sections.length - 1,
                            ariaLabel: app.attrs.tooltip("Move section down"),
                            onclick: () => moveSection(section.id, 1),
                        },
                        t.i({ className: "ri-arrow-down-line", ariaHidden: true }),
                    ),
                    t.button(
                        {
                            type: "button",
                            className: "btn sm outline circle",
                            disabled: () => local.sections.length <= 1,
                            ariaLabel: app.attrs.tooltip("Remove section"),
                            onclick: () => removeSection(section.id),
                        },
                        t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                    ),
                ),
            ),
            t.div(
                {
                    className: "record-form-layout-board",
                    "html-data-section-id": section.id,
                    onmount: (el) => el.dataset.sectionId = section.id,
                },
                section.layout.map((item) => {
                    const field = collection.fields?.find((candidate) => candidate.id == item.id);
                    return field ? createFieldCard(field, item) : null;
                }),
            ),
        );
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
        const itemsById = new Map(flattenSections(local.sections).map((item) => [item.id, item]));
        board.querySelectorAll(":scope > .record-form-layout-card").forEach((el, index) => {
            const section = local.sections.find((candidate) => candidate.id == board.dataset.sectionId);
            const item = itemsById.get(el.dataset.id) || section?.layout?.[index];
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
                const sectionsEl = el.querySelector(".record-form-layout-sections");
                const trash = el.querySelector(".record-form-layout-trash-board");
                if (!sectionsEl || !trash || sectionGrids.size || trashGrid) {
                    return;
                }

                initSectionGrids(sectionsEl);
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
                destroySectionGrids();
                trashGrid?.destroy();
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
                t.span(null, "sections: ", t.b(null, () => local.sections.length)),
                t.span(null, "fields: ", t.b(null, () => flattenSections(local.sections).length)),
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
                        { className: "record-form-layout-main-header" },
                        t.p({ className: "record-form-layout-panel-title" }, "Form canvas"),
                        t.button(
                            {
                                type: "button",
                                className: "btn sm outline",
                                onclick: () => addSection(),
                            },
                            t.i({ className: "ri-layout-row-line", ariaHidden: true }),
                            t.span({ className: "txt" }, "Add section"),
                        ),
                    ),
                    t.div(
                        { className: "record-form-layout-sections" },
                        local.sections.map((section, index) => createSectionPanel(section, index)),
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
                        { className: "record-form-layout-target" },
                        t.i({ className: "ri-add-circle-line", ariaHidden: true }),
                        t.span(
                            { className: "txt txt-ellipsis" },
                            () => getSelectedSection()?.name || "Untitled section",
                        ),
                    ),
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
                    disabled: () => local.isSaving,
                    onclick: () => resetLayout(),
                },
                t.i({ className: "ri-restart-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Reset"),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn transparent",
                    disabled: () => local.isSaving,
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Cancel"),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn",
                    disabled: () => !local.isDirty || local.isSaving,
                    onclick: () => save(),
                },
                () => local.isSaving ? t.span({ className: "loader" }) : t.span({ className: "txt" }, "Save layout"),
            ),
        ),
    );

    return modal;
}

export function layoutStorageKey(collection) {
    return app.consts.FORM_LAYOUT_STORAGE_PREFIX + collection.id;
}

export function readLayoutPreference(collection) {
    const preference = normalizePreference(collection?.rearrange);
    if (!isEmptyPreference(preference)) {
        return preference;
    }

    return normalizePreference(app.utils.getLocalHistory(layoutStorageKey(collection), null));
}

function normalizePreference(raw) {
    if (Array.isArray(raw)) {
        return { layout: raw, sections: [], hidden: [] };
    }

    return {
        layout: Array.isArray(raw?.layout) ? raw.layout : [],
        sections: Array.isArray(raw?.sections) ? raw.sections : [],
        hidden: Array.isArray(raw?.hidden) ? raw.hidden : [],
    };
}

function isEmptyPreference(preference) {
    const hasSections = preference.sections.some((section) => {
        return section?.name || section?.description || section?.layout?.length;
    });

    return !preference.layout.length && !preference.hidden.length && !hasSections;
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

export function defaultSections(collection, excludedFields = []) {
    return [{
        id: DEFAULT_SECTION_ID,
        name: "",
        description: "",
        layout: defaultLayout(collection, excludedFields),
    }];
}

export function normalizeLayout(collection, preference = {}, excludedFields = []) {
    return flattenSections(normalizeSections(collection, preference, excludedFields));
}

export function normalizeSections(collection, preference = {}, excludedFields = []) {
    const normalizedPreference = normalizePreference(preference);
    const hiddenIds = new Set(normalizedPreference.hidden);
    const fields = formFields(collection, excludedFields).filter((field) => !hiddenIds.has(field.id));
    const fieldIds = new Set(fields.map((field) => field.id));
    const used = new Set();
    const rawSections = normalizedPreference.sections.length
        ? normalizedPreference.sections
        : [{ id: DEFAULT_SECTION_ID, name: "", description: "", layout: normalizedPreference.layout }];
    const normalizedSections = [];
    const sectionIds = new Set();

    rawSections.forEach((section, index) => {
        const id = normalizeSectionId(section?.id, sectionIds, index);
        sectionIds.add(id);

        const layout = [];
        for (const item of section?.layout || []) {
            if (!fieldIds.has(item?.id) || used.has(item.id)) {
                continue;
            }

            used.add(item.id);
            layout.push(normalizeItem(item));
        }

        normalizedSections.push({
            id,
            name: `${section?.name || ""}`,
            description: `${section?.description || ""}`,
            layout: layout.sort(layoutSorter),
        });
    });

    if (!normalizedSections.length) {
        normalizedSections.push({
            id: DEFAULT_SECTION_ID,
            name: "",
            description: "",
            layout: [],
        });
    }

    const targetSection = normalizedSections[0];
    let nextY = targetSection.layout.reduce((max, item) => Math.max(max, item.y + item.h), 0);
    for (const field of fields) {
        if (used.has(field.id)) {
            continue;
        }

        targetSection.layout.push({
            id: field.id,
            x: 0,
            y: nextY++,
            w: COLUMNS,
            h: 1,
        });
    }

    targetSection.layout.sort(layoutSorter);

    return normalizedSections;
}

function serializeSections(sections = []) {
    return sections.map((section) => ({
        id: section.id,
        name: `${section.name || ""}`.trim(),
        description: `${section.description || ""}`.trim(),
        layout: (section.layout || []).map(normalizeItem).sort(layoutSorter),
    }));
}

function flattenSections(sections = []) {
    return sections.flatMap((section) => (section.layout || []).map(normalizeItem).sort(layoutSorter));
}

function normalizeSectionId(id, usedIds = new Set(), index = 0) {
    const baseId = `${id || (index ? createSectionId() : DEFAULT_SECTION_ID)}`.trim() || DEFAULT_SECTION_ID;
    let candidate = baseId;
    let counter = 2;

    while (usedIds.has(candidate)) {
        candidate = `${baseId}_${counter++}`;
    }

    return candidate;
}

function createSectionId() {
    return "section_" + app.utils.randomString(10);
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
