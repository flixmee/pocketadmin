const httpMethodOptions = [
    { value: "GET", label: "GET" },
    { value: "POST", label: "POST" },
    { value: "PUT", label: "PUT" },
    { value: "PATCH", label: "PATCH" },
    { value: "DELETE", label: "DELETE" },
];

export function httpStepForm(propsArg = {}) {
    const props = store({
        step: null,
        error: null,
    });

    const watchers = app.utils.extendStore(props, propsArg);

    return t.div(
        {
            className: "grid automation-http-step-form",
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "col-md-3" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_method` }, "Method"),
                app.components.select({
                    id: `${props.step.__id}_method`,
                    value: () => props.step.method,
                    options: httpMethodOptions,
                    onchange: (selected) => {
                        props.step.method = selected?.[0]?.value || "GET";
                    },
                }),
            ),
        ),
        t.div(
            { className: "col-md-9" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_url` }, "URL"),
                t.input({
                    id: `${props.step.__id}_url`,
                    type: "url",
                    placeholder: "https://example.com/hooks",
                    value: () => props.step.url,
                    oninput: (e) => (props.step.url = e.target.value),
                }),
            ),
        ),
        t.div(
            { className: "col-md-4" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_timeout` }, "Timeout (seconds)"),
                t.input({
                    id: `${props.step.__id}_timeout`,
                    type: "number",
                    min: 1,
                    step: 1,
                    placeholder: "30",
                    value: () => props.step.timeoutText,
                    oninput: (e) => (props.step.timeoutText = e.target.value),
                }),
            ),
            t.div({ className: "field-help" }, "Leave empty to use the default safe client timeout."),
        ),
        t.div(
            { className: "col-lg-12" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_headers` }, "Headers JSON"),
                app.components.codeEditor({
                    id: `${props.step.__id}_headers`,
                    className: "txt-code",
                    language: "js",
                    value: () => props.step.headersText,
                    placeholder: `{\n  "X-Automation": "{{trigger.type}}"\n}`,
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
                app.components.codeEditor({
                    id: `${props.step.__id}_body`,
                    className: "txt-code",
                    language: "js",
                    value: () => props.step.bodyText,
                    placeholder: `{\n  "automation": "{{automation.name}}",\n  "recordId": "{{record.id}}"\n}`,
                    oninput: (value) => (props.step.bodyText = value),
                }),
            ),
            t.div(
                { className: "field-help" },
                "Body may be plain text or JSON. Template placeholders are resolved recursively.",
            ),
        ),
    );
}
