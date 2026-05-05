const conditionOpOptions = [
    { value: "exists", label: "Exists" },
    { value: "eq", label: "Equals" },
    { value: "neq", label: "Does not equal" },
    { value: "in", label: "In list" },
];

export function conditionStepForm(propsArg = {}) {
    const props = store({
        step: null,
        error: null,
    });

    const watchers = app.utils.extendStore(props, propsArg);

    return t.div(
        {
            className: "grid automation-condition-step-form",
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "col-md-8" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_path` }, "Data path"),
                t.input({
                    id: `${props.step.__id}_path`,
                    type: "text",
                    placeholder: "record.status",
                    value: () => props.step.path,
                    oninput: (e) => (props.step.path = e.target.value),
                }),
            ),
            t.div(
                { className: "field-help" },
                "Use trigger/template paths such as ",
                t.code(null, "trigger.type"),
                ", ",
                t.code(null, "record.id"),
                ", or ",
                t.code(null, "recordOriginal.status"),
                ".",
            ),
        ),
        t.div(
            { className: "col-md-4" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_op` }, "Operator"),
                app.components.select({
                    id: `${props.step.__id}_op`,
                    value: () => props.step.op,
                    options: conditionOpOptions,
                    onchange: (selected) => {
                        props.step.op = selected?.[0]?.value || "exists";
                    },
                }),
            ),
        ),
        app.components.slide(
            () => props.step.op !== "exists",
            t.div(
                { className: "col-lg-12" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${props.step.__id}_value` }, "Expected value"),
                    app.components.codeEditor({
                        id: `${props.step.__id}_value`,
                        className: "txt-code",
                        language: "js",
                        value: () => props.step.valueText,
                        placeholder: props.step.op === "in"
                            ? `["pending", "active"]`
                            : `approved`,
                        oninput: (value) => (props.step.valueText = value),
                    }),
                ),
                t.div(
                    { className: "field-help" },
                    () =>
                        props.step.op === "in"
                            ? "Use a JSON array or a template that resolves to an array."
                            : "Strings may be entered directly; JSON values such as true, 5, or objects are also supported.",
                ),
            ),
        ),
    );
}
