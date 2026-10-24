import React, {useEffect, useState} from "react";
import savesResource from "../../../../api/resources/saves";
import Select from "../../../components/Select";
import Label from "../../../components/Label";
import {useForm} from "react-hook-form";
import Button from "../../../components/Button";
import modResource from "../../../../api/resources/mods";
import FactorioLogin from "./AddMod/components/FactorioLogin";
import ConfirmDialog from "../../../components/ConfirmDialog";

const builtInMods = ["base", "elevated-rails", "quality", "recycler", "space-age"];

const LoadMods = ({refreshMods}) => {

    const [saves, setSaves] = useState([]);
    const {register, reset, handleSubmit} = useForm();
    const [isLoading, setIsLoading] = useState(false);
    const [isDisabled, setIsDisabled] = useState(true);
    const [isFactorioAuthenticated, setIsFactorioAuthenticated] = useState(false);
    const [loadModsData, setLoadModsData] = useState(undefined);
    const [loadStatus, setLoadStatus] = useState("");

    useEffect(() => {
        (async () => {
            setIsFactorioAuthenticated(await modResource.portal.status())

            const s = await savesResource.list()
            setSaves(s);
            if (s.length > 0) {
                setIsDisabled(false);
            }
            reset();
        })();
    }, []);

    const loadModsRequested = data => {
        if (!data.save) {
            window.flash("Select a save file first.", "red");
            return;
        }
        setIsLoading(true);
        setLoadStatus("");
        setLoadModsData(data);
    }

    const loadMods = async data => {
        let step = "read";

        try {
            setLoadStatus("Reading mods from save file...");
            const saveHeader = await savesResource.mods(data.save);
            const mods = (saveHeader.mods || []).filter(mod => !builtInMods.includes(mod.name));

            step = "delete";
            setLoadStatus("Removing currently installed mods...");
            await modResource.deleteAll();

            if (mods.length === 0) {
                step = "refresh";
                setLoadStatus("Refreshing mods list...");
                await refreshMods();
                window.flash(`Save file ${data.save} does not require portal mods. Installed mods were removed.`, "green");
                return;
            }

            step = "install";
            setLoadStatus(`Downloading and installing ${mods.length} ${mods.length === 1 ? "mod" : "mods"}...`);
            await modResource.portal.installMultiple(mods);

            step = "refresh";
            setLoadStatus("Refreshing mods list...");
            await refreshMods();
            window.flash(`Mods are loaded from save file ${data.save}.`, "green");
        } catch (error) {
            if (step === "read") {
                window.flash(`Could not read mods from save file ${data.save}.`, "red");
            } else if (step === "delete") {
                window.flash(`Could not remove installed mods for save file ${data.save}.`, "red");
            } else {
                window.flash(`Could not install mods from save file ${data.save}.`, "red");
            }
        } finally {
            setIsLoading(false);
            setLoadModsData(undefined);
            setLoadStatus("");
        }
    }

    return isFactorioAuthenticated
        ? <form onSubmit={handleSubmit(loadModsRequested)}>
            <Label text="Save" htmlFor="save"/>
            <Select
                register={register('save')}
                className="mb-4"
                disabled={isDisabled}
                options={saves?.map(save => new Object({
                    name: save.name,
                    value: save.name
                }))}
            />
            <Button isSubmit={true} isDisabled={isDisabled} isLoading={isLoading}>Load</Button>
            <ConfirmDialog
                title="Load Mods from Save"
                content={<>
                    <p>{`Loading the Mods from Save "${loadModsData?.save}" will remove all currently installed Mods.`}</p>
                    {loadStatus && <p className="mt-4 text-orange font-bold">{loadStatus}</p>}
                </>}
                isOpen={loadModsData !== undefined}
                close={() => {
                    setIsLoading(false);
                    setLoadModsData(undefined);
                    setLoadStatus("");
                }}
                onSuccess={() => loadMods(loadModsData)}
            />
        </form>
        : <FactorioLogin setIsFactorioAuthenticated={setIsFactorioAuthenticated}/>
}

export default LoadMods;
