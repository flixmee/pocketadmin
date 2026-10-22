const threadTypeOptions = [
    { value: "user", label: "User (1-1 chat)" },
    { value: "group", label: "Group" },
    { value: "{{zalo.threadType}}", label: "Same as the trigger message" },
];

export function zaloStepForm(propsArg = {}) {
    const props = store({
        step: null,
        error: null,
        triggerType: "",
        triggerCollectionRef: "",
    });

    const watchers = app.utils.extendStore(props, propsArg);

    const isZaloTrigger = () => props.triggerType === "zalo.message";

    return t.div(
        {
            className: "grid automation-zalo-step-form",
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "col-md-8" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_thread_id` }, "Thread ID"),
                app.components.automationInput({
                    id: `${props.step.__id}_thread_id`,
                    singleLine: true,
                    placeholder: () => (isZaloTrigger() ? "{{zalo.threadId}}" : "User or group ID"),
                    value: () => props.step.threadId,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    oninput: (value) => (props.step.threadId = value),
                }),
            ),
        ),
        t.div(
            { className: "col-md-4" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_thread_type` }, "Thread type"),
                app.components.select({
                    id: `${props.step.__id}_thread_type`,
                    value: () => props.step.threadType,
                    options: () => {
                        const options = [...threadTypeOptions];
                        if (props.step.threadType && !options.some((o) => o.value === props.step.threadType)) {
                            options.push({ value: props.step.threadType, label: props.step.threadType });
                        }
                        return options;
                    },
                    onchange: (selected) => {
                        props.step.threadType = selected?.[0]?.value || "user";
                    },
                }),
            ),
        ),
        t.div(
            { className: "col-lg-12" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_phone` }, "Phone number"),
                app.components.automationInput({
                    id: `${props.step.__id}_phone`,
                    singleLine: true,
                    placeholder: "0901234567 (used when the thread ID is empty)",
                    value: () => props.step.phone,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    oninput: (value) => (props.step.phone = value),
                }),
            ),
            t.div(
                { className: "field-help" },
                "The person must allow being found by phone number in Zalo.",
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
                    placeholder: () => (isZaloTrigger()
                        ? "Thanks {{zalo.senderName}}, we got your message!"
                        : "Your order {{record.id}} was confirmed."),
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
                t.label({ htmlFor: `${props.step.__id}_reply_to` }, "Reply to message"),
                app.components.automationInput({
                    id: `${props.step.__id}_reply_to`,
                    singleLine: true,
                    placeholder: () => (isZaloTrigger()
                        ? "{{zalo.messageId}} (optional)"
                        : "zalo_messages record ID (optional)"),
                    value: () => props.step.replyTo,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    oninput: (value) => (props.step.replyTo = value),
                }),
            ),
            t.div({ className: "field-help" }, "Quotes the message (a zalo_messages record ID) in the reply."),
        ),
        t.div(
            { className: "col-lg-12" },
            t.div(
                { className: "field" },
                t.input({
                    id: `${props.step.__id}_wait`,
                    type: "checkbox",
                    className: "sm",
                    checked: () => props.step.waitForDelivery,
                    onchange: (e) => (props.step.waitForDelivery = e.target.checked),
                }),
                t.label(
                    { className: "txt", htmlFor: `${props.step.__id}_wait` },
                    "Wait until delivered",
                ),
            ),
            t.div(
                { className: "field-help" },
                "Otherwise the message is only queued in zalo_outbox and the step continues right away. ",
                "When waiting, the step fails if the bridge reports an error or doesn't deliver within 60 seconds.",
            ),
        ),
    );
}
