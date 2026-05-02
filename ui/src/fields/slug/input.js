export function input(props) {
    const uniqueId = "slug_" + app.utils.randomString();

    function normalizeSlug(value) {
        const asciiValue = (value || "")
            .normalize("NFD")
            .replace(/[\u0300-\u036f]/g, "")
            .replace(/[đĐ]/g, (char) => (char == "Đ" ? "D" : "d"));

        return app.utils
            .slugify(asciiValue.toLowerCase(), "-")
            .replaceAll("_", "-")
            .replace(/-+/g, "-")
            .replace(/^-+|-+$/g, "");
    }

    function getAttachedField() {
        return props.collection?.fields?.find(
            (field) => field.id == props.field.attachedField,
        );
    }

    function getAttachedFieldName() {
        return getAttachedField()?.name || props.field.attachedField;
    }

    function regenerateSlug() {
        const sourceField = getAttachedField();
        const sourceValue = sourceField
            ? props.record[sourceField.name]
            : props.record[props.field.name];

        props.record[props.field.name] = normalizeSlug(sourceValue);
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
                    placeholder: () =>
                        props.field.attachedField
                            ? `Synced from ${getAttachedFieldName()}`
                            : "",
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
                        app.components.refreshButton({
                            tooltip: () => `Generate slug from ${getAttachedFieldName()}`,
                            onclick: () => {
                                regenerateSlug();
                            },
                        }),
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
