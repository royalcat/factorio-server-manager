import React, {useEffect, useState} from "react";
import SearchModsForm from "./SearchModsForm";
import FactorioLogin from "../AddMod/components/FactorioLogin";
import modResource from "../../../../../api/resources/mods";

const SearchMods = ({refetchInstalledMods, factorioVersion}) => {
    const [isFactorioAuthenticated, setIsFactorioAuthenticated] = useState(false);

    useEffect(() => {
        (async () => {
            setIsFactorioAuthenticated(await modResource.portal.status())
        })();
    }, []);

    return isFactorioAuthenticated
        ? <SearchModsForm setIsFactorioAuthenticated={setIsFactorioAuthenticated} refetchInstalledMods={refetchInstalledMods} factorioVersion={factorioVersion} />
        : <FactorioLogin setIsFactorioAuthenticated={setIsFactorioAuthenticated}/>
}

export default SearchMods;
