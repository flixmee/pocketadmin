export function settings(props) {
    const uniqueId = "f_" + app.utils.randomString();

    const isMultipleOptions = [
        { label: "Single", value: false },
        { label: "Multiple", value: true },
    ];

    return app.components.fieldSettings(props, {
        header: [
            t.div(
                {
                    className: "field header-select single-multiple-select",
                },
                app.components.select({
                    required: true,
                    options: isMultipleOptions,
                    value: () => props.field.maxSelect > 1,
                    onchange: (opts) => {
                        if (opts?.[0]?.value) {
                            if (!props.field.maxSelect || props.field.maxSelect < 2) {
                                props.field.maxSelect = 10;
                            }
                        } else {
                            props.field.maxSelect = 1;
                        }
                    },
                }),
            ),
        ],
        content: () =>
            t.div(
                { className: "grid sm" },
                t.div(
                    { className: "col-sm-8" },
                    t.div(
                        { className: "field" },
                        t.label(
                            { htmlFor: uniqueId + ".mimeTypes" },
                            t.span({ className: "txt" }, "Allowed mime types"),
                            t.i({
                                className: "ri-information-line link-hint",
                                ariaDescription: app.attrs.tooltip(
                                    "Restrict selectable files to the listed mime types. Leave empty for no restriction.",
                                ),
                            }),
                        ),
                        app.components.select({
                            max: 99,
                            placeholder: "No restriction",
                            options: app.utils.mimeTypes.map((opt) => {
                                return {
                                    value: opt.mimeType,
                                    label: () =>
                                        t.div(
                                            { className: "inline-flex gap-10" },
                                            t.span({ className: "txt" }, opt.ext || "-"),
                                            t.small({ className: "txt-hint" }, opt.mimeType),
                                        ),
                                };
                            }),
                            name: () => `fields.${props.fieldIndex}.mimeTypes`,
                            value: () => app.utils.toArray(props.field.mimeTypes),
                            onchange: (opts) => {
                                props.field.mimeTypes = opts.map((opt) => opt.value);
                            },
                        }),
                    ),
                ),
                t.div(
                    { className: "col-sm-4", hidden: () => props.field.maxSelect << 0 < 2 },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: uniqueId + ".maxSelect" }, "Max select"),
                        t.input({
                            type: "number",
                            id: uniqueId + ".maxSelect",
                            step: 1,
                            min: 2,
                            max: Number.MAX_SAFE_INTEGER,
                            placeholder: "Default to single",
                            name: () => `fields.${props.fieldIndex}.maxSelect`,
                            value: () => props.field.maxSelect || "",
                            onchange: (e) => {
                                const maxSelect = parseInt(e.target.value, 10);
                                props.field.maxSelect = maxSelect > 1 ? maxSelect : 1;
                            },
                        }),
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
            ),
        footer: () => [
            t.div(
                { className: "field" },
                t.input({
                    className: "sm",
                    type: "checkbox",
                    id: uniqueId + ".allowFolders",
                    name: () => `fields.${props.fieldIndex}.allowFolders`,
                    checked: () => !!props.field.allowFolders,
                    onchange: (e) => (props.field.allowFolders = e.target.checked),
                }),
                t.label(
                    { htmlFor: uniqueId + ".allowFolders" },
                    t.span({ className: "txt" }, "Allow folders"),
                    t.i({
                        className: "ri-information-line link-hint",
                        ariaDescription: app.attrs.tooltip("Allows selecting folder paths in addition to files."),
                    }),
                ),
            ),
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
                    t.small({ className: "txt-hint" }, "(!='')"),
                ),
            ),
        ],
    });
}
