export function responseStepForm(propsArg = {}) {
    const props = store({
        step: null,
        error: null,
        triggerType: "",
        triggerCollectionRef: "",
    });

    const watchers = app.utils.extendStore(props, propsArg);

    return t.div(
        {
            className: "grid automation-response-step-form",
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "col-md-4" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_status` }, "Status code"),
                t.input({
                    id: `${props.step.__id}_status`,
                    type: "number",
                    min: 100,
                    max: 599,
                    step: 1,
                    placeholder: "200",
                    value: () => props.step.statusCodeText,
                    oninput: (e) => (props.step.statusCodeText = e.target.value),
                }),
            ),
        ),
        t.div(
            { className: "col-lg-12" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_headers` }, "Headers JSON"),
                app.components.automationInput({
                    id: `${props.step.__id}_headers`,
                    className: "txt-code",
                    language: "js",
                    value: () => props.step.headersText,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    placeholder: `{\n  "X-Automation": "{{automation.name}}"\n}`,
                    oninput: (value) => (props.step.headersText = value),
                }),
            ),
            t.div({ className: "field-help" }, "Headers must be a JSON object."),
        ),
        t.div(
            { className: "col-lg-12" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_body` }, "Body"),
                app.components.automationInput({
                    id: `${props.step.__id}_body`,
                    className: "txt-code",
                    language: "js",
                    value: () => props.step.bodyText,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    placeholder: `{\n  "ok": true,\n  "result": "{{prevStep.output.id}}"\n}`,
                    oninput: (value) => (props.step.bodyText = value),
                }),
            ),
            t.div(
                { className: "field-help" },
                "The webhook caller receives this response. Body may be plain text or JSON.",
            ),
        ),
    );
}
