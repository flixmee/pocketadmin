const conditionOpOptions = [
    { value: "exists", label: "Exists" },
    { value: "empty", label: "Is empty" },
    { value: "notEmpty", label: "Is not empty" },
    { value: "eq", label: "Equals" },
    { value: "neq", label: "Does not equal" },
    { value: "startsWith", label: "Starts with" },
    { value: "endsWith", label: "Ends with" },
    { value: "notStartsWith", label: "Does not start with" },
    { value: "notEndsWith", label: "Does not end with" },
    { value: "contains", label: "Contains" },
    { value: "in", label: "In list" },
];

const conditionMatchOptions = [
    { value: "and", label: "All conditions" },
    { value: "or", label: "Any condition" },
];

const conditionOpsWithoutValue = ["exists", "empty", "notEmpty"];

export function conditionStepForm(propsArg = {}) {
    const props = store({
        step: null,
        error: null,
        triggerType: "",
        triggerCollectionRef: "",
    });

    const watchers = app.utils.extendStore(props, propsArg);

    if (!Array.isArray(props.step.conditions) || !props.step.conditions.length) {
        props.step.conditions = [{
            __id: app.utils.randomString(),
            path: props.step.path || "",
            op: props.step.op || "exists",
            valueText: props.step.valueText || "",
        }];
    }

    function addCondition() {
        props.step.conditions.push({
            __id: app.utils.randomString(),
            path: "",
            op: "exists",
            valueText: "",
        });
    }

    function removeCondition(index) {
        if (props.step.conditions.length <= 1) {
            return;
        }

        props.step.conditions.splice(index, 1);
    }

    return t.div(
        {
            className: "grid automation-condition-step-form flex-start",
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "col-md-8", hidden: () => props.step.conditions.length < 2 },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_match` }, "Match"),
                app.components.select({
                    id: `${props.step.__id}_match`,
                    value: () => props.step.match || "and",
                    options: conditionMatchOptions,
                    onchange: (selected) => {
                        props.step.match = selected?.[0]?.value || "and";
                    },
                }),
            ),
        ),
        t.div(
            { className: "col-md-4" },
            t.button(
                {
                    type: "button",
                    className: "btn secondary m-t-20",
                    onclick: addCondition,
                },
                t.i({ className: "ri-add-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Add condition"),
            ),
        ),
        t.div(
            { className: "col-lg-12 automation-config-section" },
            t.div({ className: "txt-bold m-b-xs" }, "Conditions"),
            () =>
                t.div(
                    { className: "automation-config-rows" },
                    ...props.step.conditions.map((condition, index) =>
                        t.div(
                            { className: "automation-config-row schema" },
                            t.div(
                                { className: "txt-sm txt-hint txt-center" },
                                () => index === 0 ? "If" : (props.step.match || "and").toUpperCase(),
                            ),
                            t.input({
                                type: "text",
                                placeholder: "record.status",
                                value: () => condition.path,
                                oninput: (e) => (condition.path = e.target.value),
                            }),
                            app.components.select({
                                value: () => condition.op,
                                options: conditionOpOptions,
                                onchange: (selected) => {
                                    condition.op = selected?.[0]?.value || "exists";
                                },
                            }),
                            app.components.slide(
                                () => !conditionOpsWithoutValue.includes(condition.op),
                                app.components.automationInput({
                                    className: "txt-code",
                                    language: "js",
                                    value: () => condition.valueText,
                                    triggerType: () => props.triggerType,
                                    triggerCollectionRef: () => props.triggerCollectionRef,
                                    placeholder: condition.op === "in"
                                        ? `["pending", "active"]`
                                        : `approved`,
                                    oninput: (value) => (condition.valueText = value),
                                }),
                            ),
                            t.button(
                                {
                                    type: "button",
                                    className: "btn sm secondary transparent circle",
                                    disabled: () => props.step.conditions.length <= 1,
                                    ariaLabel: app.attrs.tooltip("Remove condition"),
                                    onclick: () => removeCondition(index),
                                },
                                t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                            ),
                        )
                    ),
                ),
            t.div(
                { className: "field-help m-t-xs" },
                "Use trigger/template paths such as ",
                t.code(null, "trigger.type"),
                ", ",
                t.code(null, "record.id"),
                ", or ",
                t.code(null, "recordOriginal.status"),
                ". Previous step data is available through ",
                t.code(null, "steps.0.output.id"),
                " or ",
                t.code(null, "prevStep.output.id"),
                ".",
            ),
        ),
    );
}
