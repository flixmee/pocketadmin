export function input(props) {
    const uniqueId = "slug_" + app.utils.randomString();

    function normalizeSlug(value) {
        return app.utils
            .slugify((value || "").toLowerCase(), "-")
            .replaceAll("_", "-")
            .replace(/-+/g, "-")
            .replace(/^-+|-+$/g, "");
    }

    function getAttachedFieldName() {
        return props.collection?.fields?.find((field) => field.id == props.field.attachedField)?.name
            || props.field.attachedField;
    }

    return t.div(
        { className: "record-field-input field-type-text field-type-slug" },
        t.div(
            { className: "fields" },
            t.div(
                { className: "field" },
                t.label(
                    { htmlFor: uniqueId },
                    t.i({
                        ariaHidden: true,
                        className: () => app.fieldTypes.slug.icon,
                    }),
                    t.span({ className: "txt" }, () => props.field.name),
                ),
                t.input({
                    type: "text",
                    id: uniqueId,
                    spellcheck: false,
                    name: () => props.field.name,
                    required: () => !!props.field.required,
                    placeholder: () => (props.field.attachedField ? `Synced from ${getAttachedFieldName()}` : ""),
                    value: () => props.record[props.field.name] || "",
                    oninput: (e) => {
                        props.record[props.field.name] = normalizeSlug(e.target.value);
                    },
                }),
            ),
            () => {
                if (props.field.attachedField) {
                    return t.div(
                        { className: "field addon" },
                        t.i({ className: "ri-links-line txt-hint", ariaHidden: true }),
                    );
                }
            },
        ),
        () => {
            if (props.field.help) {
                return t.div({ className: "field-help" }, props.field.help);
            }
        },
    );
}
