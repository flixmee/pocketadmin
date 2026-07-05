import { openSchemaEditorModal } from "./schemaEditorModal";
import { defaultJsonSchema, getJsonSchemaState, setJsonSchemaState } from "./schemaState";

// {
//     originalCollection: undefined,
//     collection: undefined,
//     field
//     get fieldIndex: int/-1,
//     get originalField: undefined
// }
export function settings(props) {
    const uniqueId = "f_" + app.utils.randomString();
    const initialSchemaState = getJsonSchemaState(props.field);

    const local = store({
        showInfo: false,
        schemaEnabled: initialSchemaState.enabled,
        schemaValue: initialSchemaState.value,
        showSchemaBanner: initialSchemaState.showBanner,
    });

    return app.components.fieldSettings(props, {
        content: () =>
            t.div(
                { className: "grid sm" },
                t.div(
                    { className: "col-sm-12" },
                    t.div(
                        { className: "field" },
                        t.label(
                            { htmlFor: uniqueId + ".maxSize" },
                            t.span(null, "Max size "),
                            t.small(null, "(bytes)"),
                        ),
                        t.input({
                            type: "number",
                            id: uniqueId + ".maxSize",
                            name: () => `fields.${props.fieldIndex}.maxSize`,
                            min: 0,
                            step: 1,
                            max: Number.MAX_SAFE_INTEGER,
                            placeholder: "Default to max ~1MB",
                            value: () => props.field.maxSize || "",
                            oninput: (e) => {
                                props.field.maxSize = parseInt(e.target.value, 10);
                            },
                        }),
                    ),
                ),
                // --- JSON Schema toggle + config ---
                t.div(
                    { className: "col-sm-12" },
                    t.div(
                        { className: "json-schema-section" },
                        t.div(
                            { className: "flex" },
                            t.div(
                                { className: "field m-r-auto" },
                                t.input({
                                    type: "checkbox",
                                    id: uniqueId + ".schemaEnabled",
                                    className: "sm",
                                    checked: () => local.schemaEnabled,
                                    onchange: (e) => {
                                        local.schemaEnabled = e.target.checked;

                                        if (!local.schemaEnabled) {
                                            local.schemaValue = "";
                                            local.showSchemaBanner = false;
                                        } else {
                                            local.schemaValue = local.schemaValue || defaultJsonSchema;
                                            local.showSchemaBanner = true;
                                        }

                                        setJsonSchemaState(props.field, {
                                            enabled: local.schemaEnabled,
                                            value: local.schemaValue,
                                            showBanner: local.showSchemaBanner,
                                        });
                                    },
                                }),
                                t.label(
                                    { htmlFor: uniqueId + ".schemaEnabled" },
                                    t.span({ className: "txt" }, "Schema validation"),
                                    t.i({
                                        className: "ri-information-line link-hint",
                                        ariaDescription: app.attrs.tooltip(
                                            "Validate JSON values against a JSON Schema (draft-07)",
                                        ),
                                    }),
                                ),
                            ),
                            t.button(
                                {
                                    type: "button",
                                    className: "btn sm secondary",
                                    hidden: () => !local.schemaEnabled,
                                    onclick: () => {
                                        openSchemaEditorModal({
                                            schema: local.schemaValue || "",
                                            onSave: (schemaStr) => {
                                                local.schemaValue = schemaStr;
                                                local.schemaEnabled = !!schemaStr;
                                                if (!schemaStr) {
                                                    local.showSchemaBanner = false;
                                                }
                                                setJsonSchemaState(props.field, {
                                                    enabled: local.schemaEnabled,
                                                    value: local.schemaValue,
                                                    showBanner: local.showSchemaBanner,
                                                });
                                            },
                                        });
                                    },
                                },
                                t.i({ className: "ri-settings-3-line", ariaHidden: true }),
                                t.span({ className: "txt" }, " Configure"),
                            ),
                        ),
                        // Info banner
                        t.div(
                            {
                                className: "alert info m-t-10 json-schema-banner",
                                hidden: () => !local.schemaEnabled || !local.showSchemaBanner,
                            },
                            t.div(
                                { className: "content" },
                                t.i({ className: "ri-information-line" }),
                                " Existing records are not retroactively validated. They will be validated on next save.",
                            ),
                            t.button(
                                {
                                    type: "button",
                                    className: "close",
                                    ariaLabel: "Dismiss",
                                    onclick: () => {
                                        local.showSchemaBanner = false;
                                        setJsonSchemaState(props.field, {
                                            enabled: local.schemaEnabled,
                                            value: local.schemaValue,
                                            showBanner: local.showSchemaBanner,
                                        });
                                    },
                                },
                                t.i({ className: "ri-close-line" }),
                            ),
                        ),
                    ),
                ),
                t.div(
                    { className: "col-sm-12" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: uniqueId + ".help" }, "Help text"),
                        t.input({
                            type: "text",
                            id: uniqueId + ".help",
                            name: () => `fields.${props.fieldIndex}.help`,
                            value: () => props.field.help || "",
                            oninput: (e) => (props.field.help = e.target.value),
                        }),
                    ),
                ),
                t.div(
                    { className: "col-sm-12" },
                    t.button(
                        {
                            type: "button",
                            className: () => `btn sm secondary ${local.showInfo ? "" : "transparent"}`,
                            onclick: () => (local.showInfo = !local.showInfo),
                        },
                        t.span({ className: "txt" }, "String value normalizations"),
                        t.i({
                            className: () => local.showInfo ? "ri-arrow-up-s-line" : "ri-arrow-down-s-line",
                            ariaHidden: true,
                        }),
                    ),
                    app.components.slide(
                        () => local.showInfo,
                        t.div(
                            { className: "alert m-t-10 info" },
                            t.div(
                                { className: "content" },
                                "In order to support seamlessly both ",
                                t.code(null, "application/json"),
                                " and ",
                                t.code(null, "multipart/form-data"),
                                "requests, the following normalization rules are applied if the ",
                                t.code(null, "json"),
                                " field is a plain string:",
                                t.ul(
                                    null,
                                    t.li(
                                        null,
                                        `"true" is converted to the json `,
                                        t.code(null, "true"),
                                    ),
                                    t.li(
                                        null,
                                        `"false" is converted to the json `,
                                        t.code(null, "false"),
                                    ),
                                    t.li(
                                        null,
                                        `"null" is converted to the json `,
                                        t.code(null, "null"),
                                    ),
                                    t.li(
                                        null,
                                        `"[1,2,3]" is converted to the json `,
                                        t.code(null, "[1,2,3]"),
                                    ),
                                    t.li(
                                        null,
                                        `'{"a":1,"b":2}' is converted to the json `,
                                        t.code(null, `{"a":1,"b":2}`),
                                    ),
                                    t.li(null, `numeric strings are converted to json number`),
                                    t.li(
                                        null,
                                        `double quoted strings are left as they are (aka. without normalizations)`,
                                    ),
                                    t.li(
                                        null,
                                        `any other string (empty string too) is double quoted`,
                                    ),
                                ),
                                "Alternatively, if you want to avoid the string value normalizations, you can wrap your data inside an object, eg. ",
                                t.code(null, "{\"data\": anything}"),
                                ".",
                            ),
                        ),
                    ),
                ),
            ),
        footer: () => [
            t.div(
                { className: "field" },
                t.input({
                    className: "sm",
                    type: "checkbox",
                    id: uniqueId + ".required",
                    name: () => `fields.${props.fieldIndex}.required`,
                    checked: () => !!props.field.required,
                    onchange: (e) => (props.field.required = e.target.checked),
                }),
                t.label(
                    { htmlFor: uniqueId + ".required" },
                    t.span({ className: "txt" }, "Required"),
                    t.i({
                        className: "ri-information-line link-hint",
                        ariaDescription: app.attrs.tooltip("Requires the field value NOT to be null, '', [], {}."),
                    }),
                ),
            ),
        ],
    });
}
