// Helpers to convert mod setting values between the API wire format and the
// representation used while editing.

export const editableTypes = ["bool", "int", "double", "string", "color"];

export const isEditable = node => editableTypes.includes(node.type);

const numberToInput = value => (value === null || value === undefined ? "" : String(value));

// toEditableNode prepares a node coming from the API for editing. Numbers are
// edited as strings so partially typed input like "1." is not destroyed by
// Number() coercion.
export const toEditableNode = node => {
    switch (node.type) {
        case "double":
            return {...node, value: numberToInput(node.value)};
        case "color":
            return {
                ...node,
                value: {
                    r: numberToInput(node.value?.r),
                    g: numberToInput(node.value?.g),
                    b: numberToInput(node.value?.b),
                    a: numberToInput(node.value?.a),
                },
            };
        default:
            return node;
    }
};

const isNumberInput = value => String(value).trim() !== "" && Number.isFinite(Number(value));

// toWireNode converts an edited node back into the shape the API expects.
export const toWireNode = node => {
    switch (node.type) {
        case "double":
            return {...node, value: Number(node.value)};
        case "color":
            return {
                ...node,
                value: {
                    r: Number(node.value.r),
                    g: Number(node.value.g),
                    b: Number(node.value.b),
                    a: Number(node.value.a),
                },
            };
        default:
            return node;
    }
};

export const nodeValidationError = node => {
    switch (node.type) {
        case "int":
            return /^-?\d+$/.test(String(node.value).trim()) ? null : "Enter a whole number.";
        case "double":
            return isNumberInput(node.value) ? null : "Enter a number.";
        case "color":
            return ["r", "g", "b", "a"].every(channel => isNumberInput(node.value?.[channel]))
                ? null
                : "Enter a number for the red, green, blue and alpha channels.";
        default:
            return null;
    }
};

export const nodesEqual = (a, b) => JSON.stringify(toWireNode(a)) === JSON.stringify(toWireNode(b));

export const formatNodeValue = node => {
    const wire = toWireNode(node);
    switch (wire.type) {
        case "bool":
            return wire.value ? "Enabled" : "Disabled";
        case "color":
            return `r ${wire.value.r}, g ${wire.value.g}, b ${wire.value.b}, a ${wire.value.a}`;
        case "string":
            return wire.value === null ? "(none)" : wire.value;
        default:
            return String(wire.value);
    }
};
