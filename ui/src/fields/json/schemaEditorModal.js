/**
 * JSON Schema Editor Modal
 *
 * Provides visual and raw editing modes for JSON Schema (draft-07).
 * Visual mode supports: root type, object properties + required flags,
 * array item type, string length limits, number/integer min/max.
 * Raw mode: full JSON editing with syntax highlighting.
 */

const SUPPORTED_TYPES = ["object", "array", "string", "number", "integer", "boolean"];
const PROPERTY_TYPES = ["string", "number", "integer", "boolean", "object", "array"];
const SUPPORTED_TYPE_OPTIONS = buildTypeOptions(SUPPORTED_TYPES);
const PROPERTY_TYPE_OPTIONS = buildTypeOptions(PROPERTY_TYPES);

function buildTypeOptions(types) {
    return types.map((type) => ({ value: type, label: type }));
}

function createPropertyState(prop = {}) {
    return store({
        name: prop.name || "",
        type: prop.type || "string",
        required: !!prop.required,
    });
}

/**
 * Opens the JSON Schema editor modal.
 *
 * @param {object} settings
 * @param {string} settings.schema - Current schema JSON string (or "")
 * @param {function} settings.onSave - Called with the new schema string on save
 */
export function openSchemaEditorModal(settings = {}) {
    const modal = schemaEditorModal(settings);
    document.body.appendChild(modal);
    app.modals.open(modal);
}

function schemaEditorModal(settings) {
    let modal;
    const uniqueId = "json_schema_editor_" + app.utils.randomString();

    const data = store({
        mode: "visual", // "visual" or "raw"
        rawSchema: "",
        visualUnsupported: false,

        // Visual mode state
        rootType: "object",
        rootRepeated: false,
        properties: [], // [{name, type, required}]
        arrayItemType: "string",
        stringMinLength: "",
        stringMaxLength: "",
        numberMinimum: "",
        numberMaximum: "",
    });

    // Initialize from existing schema.
    initFromSchema(settings.schema || "", data);

    function save() {
        let schemaStr = "";
        if (data.mode === "raw") {
            schemaStr = data.rawSchema.trim();
        } else {
            schemaStr = buildSchemaFromVisual(data);
        }

        // Validate JSON if non-empty.
        if (schemaStr) {
            try {
                JSON.parse(schemaStr);
            } catch (_) {
                app.toasts.error("Invalid JSON Schema");
                return;
            }
        }

        settings.onSave?.(schemaStr);
        app.modals.close(modal);
    }

    function switchMode(newMode) {
        if (newMode === data.mode) return;

        if (data.mode === "visual" && newMode === "raw") {
            // Visual -> Raw: serialize visual state to JSON.
            data.rawSchema = buildSchemaFromVisual(data);
        } else if (data.mode === "raw" && newMode === "visual") {
            // Raw -> Visual: parse JSON and populate visual state.
            if (!parseSchemaToVisual(data.rawSchema, data)) {
                data.visualUnsupported = true;
                app.toasts.info("This schema cannot be fully represented visually. Some settings may be lost.");
            } else {
                data.visualUnsupported = false;
            }
        }
        data.mode = newMode;
    }

    function addProperty() {
        data.properties = [...data.properties, createPropertyState()];
    }

    function removeProperty(index) {
        data.properties = data.properties.filter((_, i) => i !== index);
    }

    modal = t.div(
        {
            pbEvent: "jsonSchemaEditorModal",
            className: "modal popup md json-schema-modal",
            onafterclose: (el) => el?.remove(),
        },
        // Header
        t.header(
            { className: "modal-header" },
            t.h5({ className: "m-auto txt-center" }, "JSON Schema Editor"),
        ),
        // Mode tabs
        t.div(
            { className: "tabs m-b-0 json-schema-tabs" },
            t.button(
                {
                    type: "button",
                    className: () => `tab ${data.mode === "visual" ? "active" : ""}`,
                    onclick: () => switchMode("visual"),
                },
                t.i({ className: "ri-layout-grid-line", ariaHidden: true }),
                t.span({ className: "txt" }, " Visual"),
            ),
            t.button(
                {
                    type: "button",
                    className: () => `tab ${data.mode === "raw" ? "active" : ""}`,
                    onclick: () => switchMode("raw"),
                },
                t.i({ className: "ri-code-s-slash-line", ariaHidden: true }),
                t.span({ className: "txt" }, " Raw JSON"),
            ),
        ),
        // Content
        t.div(
            { className: "modal-content" },
            // Visual mode
            () => {
                if (data.mode !== "visual") return null;

                if (data.visualUnsupported) {
                    return t.div(
                        { className: "alert warning m-b-base" },
                        t.div(
                            { className: "content" },
                            t.i({ className: "ri-alert-line" }),
                            " This schema uses keywords not supported in visual mode. Switch to Raw JSON mode for full editing.",
                        ),
                    );
                }

                return t.div(
                    { className: "json-schema-visual" },
                    // Root type selector
                    t.div(
                        { className: "grid sm m-b-sm" },
                        t.div(
                            { className: "col-sm-12" },
                            t.div(
                                { className: "field" },
                                t.label({ htmlFor: uniqueId + ".rootType" }, "Root type"),
                                app.components.select({
                                    id: uniqueId + ".rootType",
                                    options: SUPPORTED_TYPE_OPTIONS,
                                    value: () => data.rootType,
                                    onchange: (opts) => {
                                        data.rootType = opts?.[0]?.value || "object";
                                        if (data.rootType !== "object") {
                                            data.rootRepeated = false;
                                        }
                                    },
                                }),
                            ),
                        ),
                        () => {
                            if (data.rootType !== "object") return null;

                            return t.div(
                                { className: "col-sm-12" },
                                t.div(
                                    { className: "field" },
                                    t.input({
                                        type: "checkbox",
                                        id: uniqueId + ".rootRepeated",
                                        className: "sm",
                                        checked: () => !!data.rootRepeated,
                                        onchange: (e) => {
                                            data.rootRepeated = e.target.checked;
                                        },
                                    }),
                                    t.label(
                                        { htmlFor: uniqueId + ".rootRepeated" },
                                        "Repeated",
                                    ),
                                ),
                            );
                        },
                    ),
                    // Object properties
                    () => {
                        if (data.rootType !== "object") return null;

                        return t.div(
                            { className: "json-schema-properties" },
                            t.div(
                                { className: "flex m-b-5" },
                                t.label({ className: "m-r-auto" }, t.strong(null, "Properties")),
                                t.button(
                                    {
                                        type: "button",
                                        className: "btn sm secondary",
                                        onclick: addProperty,
                                    },
                                    t.i({ className: "ri-add-line", ariaHidden: true }),
                                    t.span({ className: "txt" }, " Add property"),
                                ),
                            ),
                            () => {
                                if (!data.properties.length) {
                                    return t.div(
                                        { className: "txt-hint txt-center p-10" },
                                        "No properties defined. Click \"Add property\" to add one.",
                                    );
                                }

                                return data.properties.map((prop, index) =>
                                    t.div(
                                        { className: "json-schema-property-row" },
                                        t.div(
                                            { className: "field" },
                                            t.label(
                                                { className: "txt-hint txt-sm" },
                                                "Name",
                                            ),
                                            t.input({
                                                type: "text",
                                                placeholder: "Property name",
                                                value: () => prop.name,
                                                oninput: (e) => {
                                                    prop.name = e.target.value;
                                                },
                                            }),
                                        ),
                                        t.div(
                                            { className: "field" },
                                            t.label(
                                                { className: "txt-hint txt-sm" },
                                                "Type",
                                            ),
                                            app.components.select({
                                                options: PROPERTY_TYPE_OPTIONS,
                                                value: () => prop.type,
                                                onchange: (opts) => {
                                                    prop.type = opts?.[0]?.value || "string";
                                                },
                                            }),
                                        ),
                                        t.div(
                                            { className: "field" },
                                            t.input({
                                                type: "checkbox",
                                                id: uniqueId + ".prop." + index + ".required",
                                                className: "sm",
                                                checked: () => !!prop.required,
                                                onchange: (e) => {
                                                    prop.required = e.target.checked;
                                                },
                                            }),
                                            t.label(
                                                { htmlFor: uniqueId + ".prop." + index + ".required" },
                                                "Req",
                                            ),
                                        ),
                                        t.button(
                                            {
                                                type: "button",
                                                className: "btn sm circle transparent danger",
                                                onclick: () => removeProperty(index),
                                                ariaLabel: "Remove property",
                                            },
                                            t.i({ className: "ri-close-line", ariaHidden: true }),
                                        ),
                                    )
                                );
                            },
                        );
                    },
                    // Array item type
                    () => {
                        if (data.rootType !== "array") return null;

                        return t.div(
                            { className: "grid sm m-b-sm" },
                            t.div(
                                { className: "col-sm-12" },
                                t.div(
                                    { className: "field" },
                                    t.label({ htmlFor: uniqueId + ".arrayItemType" }, "Item type"),
                                    app.components.select({
                                        id: uniqueId + ".arrayItemType",
                                        options: PROPERTY_TYPE_OPTIONS,
                                        value: () => data.arrayItemType,
                                        onchange: (opts) => {
                                            data.arrayItemType = opts?.[0]?.value || "string";
                                        },
                                    }),
                                ),
                            ),
                        );
                    },
                    // String constraints
                    () => {
                        if (data.rootType !== "string") return null;

                        return t.div(
                            { className: "grid sm" },
                            t.div(
                                { className: "col-sm-6" },
                                t.div(
                                    { className: "field" },
                                    t.label({ htmlFor: uniqueId + ".stringMinLength" }, "Min length"),
                                    t.input({
                                        type: "number",
                                        id: uniqueId + ".stringMinLength",
                                        min: 0,
                                        step: 1,
                                        placeholder: "No limit",
                                        value: () => data.stringMinLength,
                                        oninput: (e) => (data.stringMinLength = e.target.value),
                                    }),
                                ),
                            ),
                            t.div(
                                { className: "col-sm-6" },
                                t.div(
                                    { className: "field" },
                                    t.label({ htmlFor: uniqueId + ".stringMaxLength" }, "Max length"),
                                    t.input({
                                        type: "number",
                                        id: uniqueId + ".stringMaxLength",
                                        min: 0,
                                        step: 1,
                                        placeholder: "No limit",
                                        value: () => data.stringMaxLength,
                                        oninput: (e) => (data.stringMaxLength = e.target.value),
                                    }),
                                ),
                            ),
                        );
                    },
                    // Number/Integer constraints
                    () => {
                        if (data.rootType !== "number" && data.rootType !== "integer") return null;

                        return t.div(
                            { className: "grid sm" },
                            t.div(
                                { className: "col-sm-6" },
                                t.div(
                                    { className: "field" },
                                    t.label({ htmlFor: uniqueId + ".numberMinimum" }, "Minimum"),
                                    t.input({
                                        type: "number",
                                        id: uniqueId + ".numberMinimum",
                                        step: data.rootType === "integer" ? 1 : "any",
                                        placeholder: "No limit",
                                        value: () => data.numberMinimum,
                                        oninput: (e) => (data.numberMinimum = e.target.value),
                                    }),
                                ),
                            ),
                            t.div(
                                { className: "col-sm-6" },
                                t.div(
                                    { className: "field" },
                                    t.label({ htmlFor: uniqueId + ".numberMaximum" }, "Maximum"),
                                    t.input({
                                        type: "number",
                                        id: uniqueId + ".numberMaximum",
                                        step: data.rootType === "integer" ? 1 : "any",
                                        placeholder: "No limit",
                                        value: () => data.numberMaximum,
                                        oninput: (e) => (data.numberMaximum = e.target.value),
                                    }),
                                ),
                            ),
                        );
                    },
                );
            },
            // Raw mode
            () => {
                if (data.mode !== "raw") return null;

                return t.div(
                    { className: "json-schema-raw" },
                    app.components.codeEditor({
                        language: "js",
                        id: uniqueId + ".rawEditor",
                        value: () => data.rawSchema,
                        oninput: (val) => (data.rawSchema = val),
                    }),
                );
            },
        ),
        // Footer
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Cancel"),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn",
                    onclick: save,
                },
                t.span({ className: "txt" }, "Save schema"),
            ),
        ),
    );

    return modal;
}

// --- Schema parsing/building helpers ---

function initFromSchema(schemaStr, data) {
    if (!schemaStr) {
        data.rawSchema = "";
        data.rootType = "object";
        data.rootRepeated = false;
        data.properties = [];
        return;
    }

    data.rawSchema = formatJSON(schemaStr);

    if (!parseSchemaToVisual(schemaStr, data)) {
        data.mode = "raw";
        data.visualUnsupported = true;
    }
}

function parseSchemaToVisual(schemaStr, data) {
    if (!schemaStr || !schemaStr.trim()) {
        data.rootType = "object";
        data.rootRepeated = false;
        data.properties = [];
        data.visualUnsupported = false;
        return true;
    }

    let schema;
    try {
        schema = JSON.parse(schemaStr);
    } catch (_) {
        return false;
    }

    if (typeof schema !== "object" || schema === null) {
        return false;
    }

    // Check for unsupported keywords.
    const supportedRootKeys = new Set([
        "$schema",
        "type",
        "properties",
        "required",
        "items",
        "minLength",
        "maxLength",
        "minimum",
        "maximum",
    ]);
    for (const key of Object.keys(schema)) {
        if (!supportedRootKeys.has(key)) {
            return false;
        }
    }

    const type = schema.type || "object";
    if (!SUPPORTED_TYPES.includes(type)) {
        return false;
    }

    data.visualUnsupported = false;

    if (type === "object") {
        data.rootType = "object";
        data.rootRepeated = false;
        if (!parseVisualObjectSchema(schema, data)) {
            return false;
        }
    } else if (type === "array") {
        if (schema.items?.type === "object") {
            if (Object.keys(schema.items).some((k) => !["type", "properties", "required"].includes(k))) {
                return false;
            }
            data.rootType = "object";
            data.rootRepeated = true;
            if (!parseVisualObjectSchema(schema.items, data)) {
                return false;
            }
        } else {
            data.rootType = "array";
            data.rootRepeated = false;
            data.arrayItemType = schema.items?.type || "string";
            if (schema.items && Object.keys(schema.items).some((k) => k !== "type")) {
                return false;
            }
        }
    } else if (type === "string") {
        data.rootType = "string";
        data.rootRepeated = false;
        data.stringMinLength = schema.minLength != null ? String(schema.minLength) : "";
        data.stringMaxLength = schema.maxLength != null ? String(schema.maxLength) : "";
    } else if (type === "number" || type === "integer") {
        data.rootType = type;
        data.rootRepeated = false;
        data.numberMinimum = schema.minimum != null ? String(schema.minimum) : "";
        data.numberMaximum = schema.maximum != null ? String(schema.maximum) : "";
    } else {
        data.rootType = type;
        data.rootRepeated = false;
    }

    return true;
}

function buildSchemaFromVisual(data) {
    if (data.rootType === "object") {
        const objectSchema = buildVisualObjectSchema(data);
        if (data.rootRepeated) {
            return JSON.stringify(
                {
                    type: "array",
                    items: objectSchema,
                },
                null,
                2,
            );
        }

        return JSON.stringify(objectSchema, null, 2);
    } else if (data.rootType === "array") {
        const schema = {
            type: data.rootType,
        };
        schema.items = { type: data.arrayItemType };
        return JSON.stringify(schema, null, 2);
    } else if (data.rootType === "string") {
        const schema = {
            type: data.rootType,
        };
        if (data.stringMinLength !== "") {
            schema.minLength = parseInt(data.stringMinLength, 10);
        }
        if (data.stringMaxLength !== "") {
            schema.maxLength = parseInt(data.stringMaxLength, 10);
        }
        return JSON.stringify(schema, null, 2);
    } else if (data.rootType === "number" || data.rootType === "integer") {
        const schema = {
            type: data.rootType,
        };
        if (data.numberMinimum !== "") {
            schema.minimum = parseFloat(data.numberMinimum);
        }
        if (data.numberMaximum !== "") {
            schema.maximum = parseFloat(data.numberMaximum);
        }
        return JSON.stringify(schema, null, 2);
    }

    return JSON.stringify({ type: data.rootType }, null, 2);
}

function parseVisualObjectSchema(schema, data) {
    const props = schema.properties || {};
    const required = new Set(schema.required || []);

    for (const [, propSchema] of Object.entries(props)) {
        if (typeof propSchema === "object" && propSchema !== null) {
            const propKeys = Object.keys(propSchema);
            if (propKeys.some((k) => k !== "type")) {
                return false;
            }
        }
    }

    data.properties = Object.entries(props).map(([name, propSchema]) => ({
        name,
        type: propSchema?.type || "string",
        required: required.has(name),
    })).map(createPropertyState);

    return true;
}

function buildVisualObjectSchema(data) {
    const schema = {
        type: "object",
    };

    if (data.properties.length > 0) {
        schema.properties = {};
        const required = [];
        for (const prop of data.properties) {
            if (!prop.name) continue;
            schema.properties[prop.name] = { type: prop.type };
            if (prop.required) {
                required.push(prop.name);
            }
        }
        if (required.length > 0) {
            schema.required = required;
        }
    }

    return schema;
}

function formatJSON(str) {
    try {
        return JSON.stringify(JSON.parse(str), null, 2);
    } catch (_) {
        return str;
    }
}
