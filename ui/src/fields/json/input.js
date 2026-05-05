const SUPPORTED_ROOT_TYPES = new Set(["object", "array", "string", "number", "integer", "boolean"]);
const SUPPORTED_PROPERTY_TYPES = new Set(["string", "number", "integer", "boolean"]);

// {
//     collection: undefined,
//     originalRecord: undefined,
//     record: undefined,
//     field: undefined,
// }
export function input(props) {
    const schema = parseInputSchema(props.field?.jsonSchema || "");

    if (schema) {
        return renderSchemaInput(props, schema);
    }

    return renderCodeEditorInput(props);
}

function renderCodeEditorInput(props) {
    const uniqueId = "json_" + app.utils.randomString();

    const local = store({
        value: stringifyEditorValue(props.record[props.field.name]),
    });

    const watchers = [
        watch(
            () => props.record[props.field.name],
            (newVal) => {
                const nextValue = stringifyEditorValue(newVal);
                if (nextValue !== local.value) {
                    local.value = nextValue;
                }
            },
        ),
    ];

    let updateRecordValueTimeoutId;

    return t.div(
        { className: "record-field-input field-type-json" },
        t.div(
            {
                className: "field",
                onunmount: () => {
                    clearTimeout(updateRecordValueTimeoutId);
                    watchers.forEach((w) => w?.unwatch());
                },
            },
            t.label(
                { htmlFor: uniqueId },
                t.i({ className: app.fieldTypes.json.icon, ariaHidden: true }),
                t.span({ className: "txt" }, () => props.field.name),
                t.span(
                    {
                        hidden: () => isValidStringifiedJSON(local.value.trim()),
                        className: "json-state",
                        ariaDescription: app.attrs.tooltip("Invalid JSON", "left"),
                    },
                    t.i({ className: "ri-error-warning-fill txt-danger", ariaHidden: true }),
                ),
                t.span(
                    {
                        hidden: () => !isValidStringifiedJSON(local.value.trim()),
                        className: "json-state",
                        ariaDescription: app.attrs.tooltip("Valid JSON", "left"),
                    },
                    t.i({ className: "ri-checkbox-circle-fill txt-success", ariaHidden: true }),
                ),
            ),
            app.components.codeEditor({
                language: "js",
                id: uniqueId,
                name: () => props.field.name,
                required: () => props.field.required,
                value: () => local.value,
                oninput: (val) => (local.value = val),
                onblur: () => updateRawRecordValue(props, local.value),
            }),
        ),
        () => renderFieldHelp(props),
    );
}

function renderSchemaInput(props, schema) {
    const uniqueId = "json_schema_input_" + app.utils.randomString();
    const local = store({
        value: cloneSchemaValue(readSchemaRootValue(props.record[props.field.name], schema)),
    });

    const watchers = [
        watch(
            () => props.record[props.field.name],
            (newVal) => {
                const nextValue = cloneSchemaValue(readSchemaRootValue(newVal, schema));
                if (!areSchemaValuesEqual(local.value, nextValue)) {
                    local.value = nextValue;
                }
            },
        ),
    ];

    function commit() {
        props.record[props.field.name] = serializeSchemaRootValue(schema, cloneSchemaValue(local.value));
    }

    return t.div(
        {
            className: "record-field-input field-type-json",
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "field" },
            t.label(
                { htmlFor: uniqueId },
                t.i({ className: app.fieldTypes.json.icon, ariaHidden: true }),
                t.span({ className: "txt" }, () => props.field.name),
                t.span(
                    {
                        className: "json-schema-input-badge",
                        ariaDescription: app.attrs.tooltip("Schema-driven JSON input", "left"),
                    },
                    t.i({ className: "ri-layout-grid-line", ariaHidden: true }),
                ),
            ),
            t.div(
                { className: "json-schema-input" },
                renderSchemaValueInput({
                    id: uniqueId,
                    schema,
                    getValue: () => local.value,
                    setValue: (value) => {
                        local.value = cloneSchemaValue(value);
                    },
                    commit,
                    required: () => !!props.field.required,
                }),
            ),
        ),
        () => renderFieldHelp(props),
    );
}

function renderSchemaValueInput(config) {
    switch (config.schema.type) {
        case "object":
            return renderObjectInput(config);
        case "array":
            return renderArrayInput(config);
        case "string":
            return renderStringInput(config);
        case "number":
        case "integer":
            return renderNumberInput(config);
        case "boolean":
            return renderBooleanInput(config);
        default:
            return null;
    }
}

function renderObjectInput(config) {
    return t.div(
        { className: "json-schema-group" },
        config.schema.properties.map((property) => {
            const propertyId = `${config.id}.${property.name}`;

            return t.div(
                { className: "field" },
                t.label(
                    { htmlFor: propertyId },
                    t.span({ className: "txt" }, property.name),
                    property.required ? t.span({ className: "txt-danger" }, " *") : null,
                ),
                renderSchemaValueInput({
                    id: propertyId,
                    schema: property.schema,
                    getValue: () => {
                        const value = config.getValue();
                        return value?.[property.name];
                    },
                    setValue: (propertyValue) => {
                        const nextValue = {
                            ...ensureObjectValue(config.getValue()),
                        };

                        if (typeof propertyValue === "undefined") {
                            delete nextValue[property.name];
                        } else {
                            nextValue[property.name] = propertyValue;
                        }

                        config.setValue(nextValue);
                    },
                    commit: config.commit,
                    required: () => property.required,
                }),
            );
        }),
    );
}

function renderArrayInput(config) {
    return t.div(
        { className: "json-schema-array" },
        () => {
            const value = ensureArrayValue(config.getValue());

            if (!value.length) {
                return t.div({ className: "txt-hint p-10 txt-center" }, "No items added.");
            }

            return value.map((_, index) =>
                t.div(
                    { className: "json-schema-array-item" },
                    t.div({ className: "json-schema-array-index" }, index + 1),
                    t.div(
                        { className: "json-schema-array-value" },
                        renderSchemaValueInput({
                            id: `${config.id}.${index}`,
                            schema: config.schema.items,
                            getValue: () => ensureArrayValue(config.getValue())[index],
                            setValue: (itemValue) => {
                                const nextValue = ensureArrayValue(config.getValue()).slice();
                                nextValue[index] = itemValue;
                                config.setValue(nextValue);
                            },
                            commit: config.commit,
                            required: () => true,
                        }),
                    ),
                    t.button(
                        {
                            type: "button",
                            className: "btn sm circle transparent danger",
                            onclick: () => {
                                const nextValue = ensureArrayValue(config.getValue()).slice();
                                nextValue.splice(index, 1);
                                config.setValue(nextValue);
                                config.commit?.();
                            },
                            ariaLabel: "Remove item",
                        },
                        t.i({ className: "ri-close-line", ariaHidden: true }),
                    ),
                )
            );
        },
        t.button(
            {
                type: "button",
                className: "btn sm secondary m-t-5",
                onclick: () => {
                    const nextValue = ensureArrayValue(config.getValue()).slice();
                    nextValue.push(getDefaultValue(config.schema.items));
                    config.setValue(nextValue);
                    config.commit?.();
                },
            },
            t.i({ className: "ri-add-line", ariaHidden: true }),
            t.span({ className: "txt" }, " Add item"),
        ),
    );
}

function renderStringInput(config) {
    const local = store({
        value: ensureStringValue(config.getValue()),
    });

    const watchers = [
        watch(
            () => config.getValue(),
            (newVal) => {
                const nextValue = ensureStringValue(newVal);
                if (local.value !== nextValue) {
                    local.value = nextValue;
                }
            },
        ),
    ];

    return t.input({
        type: "text",
        id: config.id,
        required: () => !!config.required?.(),
        minLength: () => config.schema.minLength,
        maxLength: () => config.schema.maxLength,
        value: () => local.value,
        oninput: (e) => (local.value = e.target.value),
        onblur: () => {
            config.setValue(local.value);
            config.commit?.();
        },
        onunmount: () => {
            watchers.forEach((w) => w?.unwatch());
        },
    });
}

function renderNumberInput(config) {
    const local = store({
        value: ensureNumberTextValue(config.getValue()),
    });

    const watchers = [
        watch(
            () => config.getValue(),
            (newVal) => {
                const nextValue = ensureNumberTextValue(newVal);
                if (local.value !== nextValue) {
                    local.value = nextValue;
                }
            },
        ),
    ];

    return t.input({
        type: "number",
        id: config.id,
        required: () => !!config.required?.(),
        step: config.schema.type === "integer" ? 1 : "any",
        min: () => config.schema.minimum,
        max: () => config.schema.maximum,
        value: () => local.value,
        oninput: (e) => {
            local.value = e.target.value;
        },
        onblur: () => {
            if (local.value === "") {
                config.setValue(undefined);
                config.commit?.();
                return;
            }

            const nextValue = config.schema.type === "integer"
                ? parseInt(local.value, 10)
                : parseFloat(local.value);

            config.setValue(Number.isNaN(nextValue) ? undefined : nextValue);
            config.commit?.();
        },
        onunmount: () => {
            watchers.forEach((w) => w?.unwatch());
        },
    });
}

function renderBooleanInput(config) {
    return t.div(
        { className: "field" },
        t.input({
            type: "checkbox",
            id: config.id,
            className: "sm",
            checked: () => !!config.getValue(),
            onchange: (e) => {
                config.setValue(e.target.checked);
                config.commit?.();
            },
        }),
        t.label({ htmlFor: config.id }, "Enabled"),
    );
}

function renderFieldHelp(props) {
    if (props.field.help) {
        return t.div({ className: "field-help" }, props.field.help);
    }
}

function updateRawRecordValue(props, value) {
    const trimmed = value.trim();

    if (trimmed === "") {
        props.record[props.field.name] = null;
        return;
    }

    try {
        const parsed = JSON.parse(trimmed);
        if (typeof parsed === "string") {
            props.record[props.field.name] = JSON.stringify(parsed);
        } else {
            props.record[props.field.name] = parsed;
        }
    } catch (_) {
        props.record[props.field.name] = trimmed;
    }
}

function stringifyEditorValue(value) {
    if (typeof value === "string" && !value.startsWith("\"") && !value.endsWith("\"")) {
        return JSON.stringify(typeof value === "undefined" ? null : value);
    }

    if (typeof value === "string" && value.startsWith("\"") && value.endsWith("\"")) {
        return value;
    }

    if (value === null || typeof value === "undefined") {
        return "null";
    }

    return JSON.stringify(value, null, 2);
}

function parseInputSchema(schemaStr) {
    if (!schemaStr || !schemaStr.trim()) {
        return null;
    }

    let schema;
    try {
        schema = JSON.parse(schemaStr);
    } catch (_) {
        return null;
    }

    return parseSchemaNode(schema, true);
}

function parseSchemaNode(schema, isRoot = false, allowObjectArrayItems = false) {
    if (typeof schema !== "object" || schema === null) {
        return null;
    }

    const type = schema.type || (isRoot ? "object" : "string");

    if (type === "object") {
        if (!isRoot && !allowObjectArrayItems) {
            return null;
        }

        const supportedKeys = new Set(["$schema", "type", "properties", "required"]);
        if (!hasOnlySupportedKeys(schema, supportedKeys)) {
            return null;
        }

        const properties = Object.entries(schema.properties || {}).map(([name, propertySchema]) => {
            const parsedProperty = parseSchemaNode(propertySchema, false, false);
            if (!parsedProperty) {
                return null;
            }

            return {
                name,
                required: (schema.required || []).includes(name),
                schema: parsedProperty,
            };
        });

        if (properties.some((property) => !property)) {
            return null;
        }

        return {
            type,
            properties,
        };
    }

    const allowedTypes = isRoot ? SUPPORTED_ROOT_TYPES : SUPPORTED_PROPERTY_TYPES;
    if (!allowedTypes.has(type)) {
        return null;
    }

    if (type === "array") {
        const supportedKeys = new Set(["$schema", "type", "items"]);
        if (!hasOnlySupportedKeys(schema, supportedKeys)) {
            return null;
        }

        const items = parseSchemaNode(schema.items || { type: "string" }, false, isRoot);
        if (!items) {
            return null;
        }

        return {
            type,
            items,
        };
    }

    if (type === "string") {
        const supportedKeys = new Set(["$schema", "type", "minLength", "maxLength"]);
        if (!hasOnlySupportedKeys(schema, supportedKeys)) {
            return null;
        }

        return {
            type,
            minLength: schema.minLength,
            maxLength: schema.maxLength,
        };
    }

    if (type === "number" || type === "integer") {
        const supportedKeys = new Set(["$schema", "type", "minimum", "maximum"]);
        if (!hasOnlySupportedKeys(schema, supportedKeys)) {
            return null;
        }

        return {
            type,
            minimum: schema.minimum,
            maximum: schema.maximum,
        };
    }

    if (!hasOnlySupportedKeys(schema, new Set(["$schema", "type"]))) {
        return null;
    }

    return { type };
}

function hasOnlySupportedKeys(obj, allowedKeys) {
    return Object.keys(obj).every((key) => allowedKeys.has(key));
}

function readSchemaRootValue(rawValue, schema) {
    if (schema.type === "string") {
        return ensureStringValue(parseJsonValue(rawValue));
    }

    if (schema.type === "number" || schema.type === "integer") {
        return ensureNumericValue(parseJsonValue(rawValue));
    }

    if (schema.type === "boolean") {
        return !!parseJsonValue(rawValue);
    }

    if (schema.type === "object") {
        return ensureObjectValue(parseJsonValue(rawValue));
    }

    if (schema.type === "array") {
        return ensureArrayValue(parseJsonValue(rawValue));
    }

    return rawValue;
}

function serializeSchemaRootValue(schema, value) {
    if (typeof value === "undefined") {
        return null;
    }

    if (schema.type === "string") {
        return JSON.stringify(value ?? "");
    }

    return value;
}

function parseJsonValue(value) {
    if (typeof value !== "string") {
        return value;
    }

    try {
        return JSON.parse(value);
    } catch (_) {
        return value;
    }
}

function cloneSchemaValue(value) {
    if (typeof value === "undefined") {
        return undefined;
    }

    return JSON.parse(JSON.stringify(value));
}

function areSchemaValuesEqual(left, right) {
    return JSON.stringify(left) === JSON.stringify(right);
}

function ensureObjectValue(value) {
    return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

function ensureArrayValue(value) {
    return Array.isArray(value) ? value : [];
}

function ensureStringValue(value) {
    return typeof value === "string" ? value : "";
}

function ensureNumericValue(value) {
    return typeof value === "number" && !Number.isNaN(value) ? value : undefined;
}

function ensureNumberTextValue(value) {
    const num = ensureNumericValue(value);
    return typeof num === "number" ? String(num) : "";
}

function getDefaultValue(schema) {
    switch (schema.type) {
        case "object":
            return {};
        case "string":
            return "";
        case "number":
        case "integer":
            return 0;
        case "boolean":
            return false;
        default:
            return null;
    }
}

function isValidStringifiedJSON(val) {
    if (val === "") {
        return true;
    }

    try {
        JSON.parse(val);
        return true;
    } catch (_) {
        return false;
    }
}
