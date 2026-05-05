const jsonSchemaState = new WeakMap();

export const defaultJsonSchema = "{\"type\":\"object\"}";

export function getJsonSchemaState(field) {
    let state = jsonSchemaState.get(field);

    if (!state) {
        state = {
            enabled: !!field?.jsonSchema,
            value: field?.jsonSchema || "",
            showBanner: !!field?.jsonSchema,
        };

        jsonSchemaState.set(field, state);
    }

    return state;
}

export function setJsonSchemaState(field, patch) {
    const state = getJsonSchemaState(field);
    Object.assign(state, patch);

    // trigger reactive updates by reset schemaEnabled
    field.schemaEnabled = field.schemaEnabled ? false : true;

    return state;
}

export function cloneCollectionWithJsonSchemaState(collection) {
    const clone = JSON.parse(JSON.stringify(collection || {}));
    const sourceFields = collection?.fields || [];
    const targetFields = clone.fields || [];

    for (let i = 0; i < targetFields.length; i++) {
        const sourceField = sourceFields[i];
        const targetField = targetFields[i];
        const state = sourceField ? jsonSchemaState.get(sourceField) : null;

        if (!state) {
            continue;
        }

        targetField.jsonSchema = state.enabled ? (state.value || "") : "";
    }

    return clone;
}
