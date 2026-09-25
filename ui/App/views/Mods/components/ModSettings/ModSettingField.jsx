import React from "react";
import Input from "../../../../components/Input";

const ModSettingField = ({node, disabled = false, onChange}) => {
    const update = value => onChange({...node, value});

    switch (node.type) {
        case "bool":
            return (
                <label className="inline-flex items-center text-white cursor-pointer">
                    <input
                        type="checkbox"
                        className="mr-2 leading-tight"
                        checked={node.value === true}
                        disabled={disabled}
                        onChange={event => update(event.target.checked)}
                    />
                    <span className="text-sm">{node.value === true ? "Enabled" : "Disabled"}</span>
                </label>
            );
        case "int":
            return (
                <Input type="number" step={1} value={node.value} disabled={disabled}
                       onChange={event => update(event.target.value)}/>
            );
        case "double":
            return (
                <Input type="number" step="any" value={node.value} disabled={disabled}
                       onChange={event => update(event.target.value)}/>
            );
        case "string":
            return (
                <Input type="text" value={node.value === null ? "" : node.value} disabled={disabled}
                       hasAutoComplete={false} onChange={event => update(event.target.value)}/>
            );
        case "color":
            return (
                <div className="flex flex-wrap items-center">
                    {["r", "g", "b", "a"].map(channel => (
                        <div key={channel} className="flex items-center mr-3 mb-1">
                            <span className="text-gray-light text-xs mr-1 uppercase">{channel}</span>
                            <div className="w-20">
                                <Input type="number" step="any" value={node.value[channel]} disabled={disabled}
                                       onChange={event => update({...node.value, [channel]: event.target.value})}/>
                            </div>
                        </div>
                    ))}
                </div>
            );
        default:
            return (
                <pre className="text-gray-light text-xs overflow-x-auto whitespace-pre-wrap">
                    {JSON.stringify(node.value, null, 2)}
                </pre>
            );
    }
};

export default ModSettingField;
