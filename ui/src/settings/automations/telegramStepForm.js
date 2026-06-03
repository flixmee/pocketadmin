const parseModeOptions = [
    { value: "", label: "Plain text" },
    { value: "Markdown", label: "Markdown" },
    { value: "MarkdownV2", label: "MarkdownV2" },
    { value: "HTML", label: "HTML" },
];

export function telegramStepForm(propsArg = {}) {
    const props = store({
        step: null,
        error: null,
        triggerType: "",
        triggerCollectionRef: "",
    });

    const watchers = app.utils.extendStore(props, propsArg);

    return t.div(
        {
            className: "grid automation-telegram-step-form",
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "col-md-8" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_chat_id` }, "Chat ID"),
                app.components.automationInput({
                    id: `${props.step.__id}_chat_id`,
                    singleLine: true,
                    placeholder: "123456789 or @channelusername",
                    value: () => props.step.chatId,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    oninput: (value) => (props.step.chatId = value),
                }),
            ),
        ),
        t.div(
            { className: "col-md-4" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_parse_mode` }, "Parse mode"),
                app.components.select({
                    id: `${props.step.__id}_parse_mode`,
                    value: () => props.step.parseMode,
                    options: parseModeOptions,
                    onchange: (selected) => {
                        props.step.parseMode = selected?.[0]?.value || "";
                    },
                }),
            ),
        ),
        t.div(
            { className: "col-lg-12" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_text` }, "Message"),
                app.components.automationInput({
                    id: `${props.step.__id}_text`,
                    className: "pre-wrap",
                    placeholder: "New record {{record.id}} was created.",
                    value: () => props.step.text,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    oninput: (value) => (props.step.text = value),
                }),
            ),
        ),
        t.div(
            { className: "col-lg-12" },
            t.div(
                { className: "field" },
                t.input({
                    id: `${props.step.__id}_disable_preview`,
                    type: "checkbox",
                    className: "sm",
                    checked: () => props.step.disableWebPagePreview,
                    onchange: (e) => (props.step.disableWebPagePreview = e.target.checked),
                }),
                t.label(
                    { className: "txt", htmlFor: `${props.step.__id}_disable_preview` },
                    "Disable web page preview",
                ),
            ),
        ),
    );
}
