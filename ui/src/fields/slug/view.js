export function view(props) {
    return t.div({ className: "record-field-view field-type-text field-type-slug" }, () => {
        const value = props.record[props.field.name] || "";

        if (value == "") {
            return t.span({ className: "missing-value" });
        }

        if (props.short) {
            return t.span({
                className: "txt txt-ellipsis",
                textContent: app.utils.truncate(value),
            });
        }

        return t.span(
            { className: "label" },
            app.components.copyButton(value),
            t.span({ className: "txt-ellipsis" }, app.utils.truncate(value)),
        );
    });
}
