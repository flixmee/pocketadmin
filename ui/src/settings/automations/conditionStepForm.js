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

    function ensureConditions() {
        if (!props.step) {
            return [];
        }

        if (!Array.isArray(props.step.conditions) || !props.step.conditions.length) {
            props.step.conditions = [{
                __id: app.utils.randomString(),
                path: props.step.path || "",
                op: props.step.op || "exists",
                valueText: props.step.valueText || "",
            }];
        }

        return props.step.conditions;
    }

    function addCondition(match) {
        const conditions = ensureConditions();
        props.step.match = match;
        conditions.push({
            __id: app.utils.randomString(),
            path: "",
            op: "exists",
            valueText: "",
        });
    }

    function removeCondition(index) {
        const conditions = ensureConditions();
        if (conditions.length <= 1) {
            return;
        }

        conditions.splice(index, 1);
    }

    function logicLabel(index) {
        if (index === 0) {
            return "IF";
        }

        return (props.step.match || "and").toUpperCase();
    }

    return t.div(
        {
            className: "automation-condition-step-form",
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "automation-condition-card" },
            t.div(
                { className: "automation-condition-header" },
                t.div(
                    { className: "automation-condition-title" },
                    t.i({ className: "ri-filter-3-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "Conditions"),
                ),
                t.div(
                    {
                        className: "automation-condition-match",
                        hidden: () => ensureConditions().length < 2,
                    },
                    app.components.select({
                        required: true,
                        searchThreshold: 99,
                        className: "automation-condition-match-select",
                        value: () => props.step.match || "and",
                        options: conditionMatchOptions,
                        onchange: (selected) => {
                            props.step.match = selected?.[0]?.value || "and";
                        },
                    }),
                ),
            ),
            () =>
                t.div(
                    { className: "automation-condition-rows" },
                    ...ensureConditions().map((condition, index) => [
                        t.div(
                            { className: "automation-condition-row" },
                            t.div(
                                {
                                    className: () =>
                                        `automation-logic-badge ${(index === 0 ? "if" : props.step.match || "and")}`,
                                },
                                () => logicLabel(index),
                            ),
                            t.input({
                                type: "text",
                                className: "automation-condition-path-input",
                                placeholder: "record.status",
                                value: () => condition.path,
                                oninput: (e) => (condition.path = e.target.value),
                            }),
                            app.components.select({
                                required: true,
                                searchThreshold: 99,
                                className: "automation-condition-op-select",
                                value: () => condition.op,
                                options: conditionOpOptions,
                                onchange: (selected) => {
                                    condition.op = selected?.[0]?.value || "exists";
                                },
                            }),
                            app.components.slide(
                                () => !conditionOpsWithoutValue.includes(condition.op),
                                app.components.automationInput({
                                    className: "txt-code automation-condition-value-input",
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
                                    className: "btn sm transparent circle automation-condition-delete-btn",
                                    disabled: () => ensureConditions().length <= 1,
                                    ariaLabel: app.attrs.tooltip("Remove condition"),
                                    onclick: () => removeCondition(index),
                                },
                                t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                            ),
                        ),
                    ]).flat().filter(Boolean),
                ),
            t.div(
                { className: "automation-condition-footer" },
                t.button(
                    {
                        type: "button",
                        className: "automation-condition-add-btn",
                        onclick: () => addCondition("and"),
                    },
                    t.i({ className: "ri-add-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "Add AND condition"),
                ),
                t.button(
                    {
                        type: "button",
                        className: "automation-condition-add-btn",
                        onclick: () => addCondition("or"),
                    },
                    t.i({ className: "ri-node-tree", ariaHidden: true }),
                    t.span({ className: "txt" }, "Add OR group"),
                ),
            ),
        ),
        t.div(
            { className: "automation-condition-hint" },
            t.div(
                { className: "automation-condition-hint-text" },
                "Use template paths such as ",
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
