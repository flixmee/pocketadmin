import { toDeleteProp } from "@/base/fieldSettings";

export function settings(props) {
    const uniqueId = "f_" + app.utils.randomString();

    function getAttachmentOptions() {
        return (props.collection?.fields || [])
            .filter((field) => !field[toDeleteProp] && field.name != props.field.name)
            .map((field) => {
                return {
                    value: field.id,
                    label: () =>
                        t.div(
                            { className: "inline-flex gap-10" },
                            t.span({ className: "txt" }, field.name),
                        ),
                };
            });
    }

    return app.components.fieldSettings(props, {
        header: t.div(
            {
                className: "field header-select slug-attach-select",
                ariaDescription: app.attrs.tooltip(
                    "Source field for slug generation",
                    "left",
                ),
            },
            app.components.select({
                placeholder: "No attachment",
                options: getAttachmentOptions(),
                value: () => props.field.attachedField || "",
                onchange: (opts) => {
                    props.field.attachedField = opts?.[0]?.value || "";
                },
            }),
        ),
        content: () =>
            t.div(
                { className: "grid sm" },
                t.div(
                    { className: "col-sm-6" },
                    t.div(
                        { className: "field" },
                        t.label(
                            { htmlFor: uniqueId + ".min" },
                            t.span({ className: "txt" }, "Min length"),
                            t.i({
                                className: "ri-information-line link-hint",
                                ariaDescription: app.attrs.tooltip(
                                    "Clear the field or set it to 0 for no limit.",
                                ),
                            }),
                        ),
                        t.input({
                            type: "number",
                            id: uniqueId + ".min",
                            name: () => `fields.${props.fieldIndex}.min`,
                            step: 1,
                            min: 0,
                            max: Number.MAX_SAFE_INTEGER,
                            placeholder: "No min limit",
                            value: () => props.field.min || "",
                            oninput: (e) => {
                                props.field.min = parseInt(e.target.value, 10);
                            },
                        }),
                    ),
                ),
                t.div(
                    { className: "col-sm-6" },
                    t.div(
                        { className: "field" },
                        t.label(
                            { htmlFor: uniqueId + ".max" },
                            t.span({ className: "txt" }, "Max length"),
                            t.i({
                                className: "ri-information-line link-hint",
                                ariaDescription: app.attrs.tooltip(
                                    "Clear the field or set it to 0 to fallback to the default limit.",
                                ),
                            }),
                        ),
                        t.input({
                            type: "number",
                            id: uniqueId + ".max",
                            name: () => `fields.${props.fieldIndex}.max`,
                            step: 1,
                            min: () => props.field.min || 0,
                            max: Number.MAX_SAFE_INTEGER,
                            placeholder: "Default to max 5000 characters",
                            value: () => props.field.max || "",
                            oninput: (e) => {
                                props.field.max = parseInt(e.target.value, 10);
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
                    id: uniqueId + ".required",
                    name: () => `fields.${props.fieldIndex}.required`,
                    checked: () => !!props.field.required,
                    onchange: (e) => (props.field.required = e.target.checked),
                }),
                t.label(
                    { htmlFor: uniqueId + ".required" },
                    t.span({ className: "txt" }, "Required"),
                    t.small({ className: "txt-hint" }, "(!='')"),
                    t.i({
                        className: "ri-information-line link-hint",
                        ariaDescription: app.attrs.tooltip(
                            "Requires the field value to be nonempty string",
                        ),
                    }),
                ),
            ),
        ],
    });
}
