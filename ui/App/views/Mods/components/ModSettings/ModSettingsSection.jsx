import React, {useState} from "react";
import Input from "../../../../components/Input";
import ModSettingField from "./ModSettingField";

const ModSettingsSection = ({entries, onChange, onRevert}) => {
    const [filter, setFilter] = useState("");
    const [modifiedOnly, setModifiedOnly] = useState(false);

    const normalizedFilter = filter.trim().toLowerCase();
    const visibleEntries = entries.filter(entry =>
        (!modifiedOnly || entry.modified) &&
        (normalizedFilter === "" || entry.name.toLowerCase().includes(normalizedFilter))
    );

    return (
        <div>
            <div className="flex flex-wrap items-center mb-4">
                <div className="w-64 mr-3 mb-2">
                    <Input type="text" placeholder="Filter settings" value={filter}
                           hasAutoComplete={false}
                           onChange={event => setFilter(event.target.value)}/>
                </div>
                <label className="flex items-center text-sm text-white mr-3 mb-2 cursor-pointer">
                    <input type="checkbox" className="mr-2 leading-tight" checked={modifiedOnly}
                           onChange={event => setModifiedOnly(event.target.checked)}/>
                    Show modified only
                </label>
                <span className="text-gray-light text-sm mb-2">
                    {visibleEntries.length} / {entries.length} settings
                </span>
            </div>

            {visibleEntries.length === 0
                ? <div className="text-gray-light">No settings to display.</div>
                : visibleEntries.map(entry => (
                    <div key={entry.name} className="flex items-start border-b border-gray-medium py-3">
                        <div className="w-88 mr-4">
                            <div className="text-dirty-white font-mono text-sm break-all">{entry.name}</div>
                            <div className="text-gray-light text-xs mt-1">
                                {entry.type}{entry.readOnly ? " · read-only" : ""}
                            </div>
                        </div>
                        <div className="flex-1">
                            <ModSettingField node={entry.node} disabled={entry.readOnly}
                                             onChange={node => onChange(entry.name, node)}/>
                            {entry.error
                                ? <div className="text-red text-xs mt-1">{entry.error}</div>
                                : null}
                            {entry.modified
                                ? <div className="text-gray-light text-xs mt-1">Was: {entry.originalDisplay}</div>
                                : null}
                        </div>
                        <div className="w-20 text-right">
                            {entry.modified
                                ? <button type="button" className="text-orange text-xs hover:underline"
                                          onClick={() => onRevert(entry.name)}>Revert</button>
                                : null}
                        </div>
                    </div>
                ))
            }
        </div>
    );
};

export default ModSettingsSection;
