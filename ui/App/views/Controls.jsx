import React, {useEffect, useState} from "react";
import Panel from "../components/Panel";
import Button from "../components/Button";
import server from "../../api/resources/server";
import savesResource from "../../api/resources/saves";
import {useForm} from "react-hook-form";
import Select from "../components/Select";
import Input from "../components/Input";
import Error from "../components/Error";
import {formatFactorioVersion} from "../utils/version";

const Controls = ({serverStatus}) => {

    const factorioVersion = formatFactorioVersion(serverStatus.fac_version);
    const [saves, setSaves] = useState([]);
    const [isDisabled, setIsDisabled] = useState(true);
    const [isStopping, setIsStopping] = useState(false);
    const [isStarting, setIsStarting] = useState(false);
    const [isKilling, setIsKilling] = useState(false);
    const [backups, setBackups] = useState([]);
    const [installStatus, setInstallStatus] = useState({
        installed: serverStatus.installed,
        installing: false,
        version: factorioVersion,
        base_mod_version: formatFactorioVersion(serverStatus.base_mod_version),
        phase: 'idle',
        message: 'Ready',
        downloaded: 0,
        total: 0,
        latest_stable: '',
        latest: '',
        update_available: false,
        update_version: '',
    });
    const [installVersionMode, setInstallVersionMode] = useState('stable');
    const [customInstallVersion, setCustomInstallVersion] = useState('');
    const [isInstalling, setIsInstalling] = useState(false);
    const [installError, setInstallError] = useState('');
    const [startError, setStartError] = useState('');
    const [lifecycle, setLifecycle] = useState(null);
    const [interfaces, setInterfaces] = useState([]);
    const [isSavingLifecycle, setIsSavingLifecycle] = useState(false);

    const { handleSubmit, reset, register, formState: {errors} } = useForm();

    const startServer = async (data) => {
        setIsStarting(true);
        setStartError('');
        try {
            await server.start(data.ip, parseInt(data.port), data.save);
        } catch (error) {
            const message = error?.response?.data || error.message;
            setStartError(message);
            window.flash(message, "red");
        } finally {
            setIsStarting(false);
        }
    }

    const stopServer = async () => {
        setIsStopping(true);
        await server.stop();
    }

    const killServer = async () => {
        setIsKilling(true);
        await server.kill();
    }

    const installFactorio = async () => {
        const version = installVersionMode === 'custom' ? customInstallVersion : installVersionMode;
        await installFactorioVersion(version);
    }

    const installFactorioVersion = async (version) => {
        setIsInstalling(true);
        setInstallError('');
        setInstallStatus({
            ...installStatus,
            installing: true,
            target_version: version,
            phase: 'starting',
            message: 'Starting install',
            downloaded: 0,
            total: 0,
        });
        try {
            const status = await server.install(version);
            const [freshStatus, freshSaves, freshBackups] = await Promise.all([
                server.installStatus(),
                savesResource.list(true),
                savesResource.backups(),
            ]);
            setInstallStatus(freshStatus || status);
            setSaves(freshSaves);
            setBackups(freshBackups);
            setIsDisabled((freshStatus || status).installed && freshSaves.length > 0 ? undefined : true);
        } catch (error) {
            setInstallError(error?.response?.data || error.message);
        } finally {
            setIsInstalling(false);
        }
    }

    const updateFactorio = async () => {
        await installFactorioVersion(installStatus.update_version);
    }

    useEffect(() => {
        Promise.all([
            server.installStatus(),
            savesResource.list(true),
            savesResource.backups(),
            server.lifecycle(),
            server.interfaces().catch(() => []),
        ])
            .then(([status, saveRes, backupRes, lifecycleRes, interfaceRes]) => {
                setInstallStatus(status);
                setSaves(saveRes);
                setBackups(backupRes);
                setLifecycle(lifecycleRes);
                setInterfaces(interfaceRes);
                if (status.installed && saveRes.length > 0) {
                    setIsDisabled(undefined);
                } else {
                    setIsDisabled(true);
                }
                reset({
                    ip: lifecycleRes?.startup_profile?.bindip || "0.0.0.0",
                    port: lifecycleRes?.startup_profile?.port || 34197,
                    save: lifecycleRes?.startup_profile?.savefile || saveRes.find((save) => save.name.startsWith('Load Latest'))?.name,
                });
            })
    }, [])

    useEffect(() => {
        if (!installStatus.installing && !isInstalling) {
            return;
        }

        const interval = setInterval(() => {
            server.installStatus()
                .then(status => {
                    setInstallStatus(status);
                    if (!status.installing) {
                        setIsInstalling(false);
                        setIsDisabled(status.installed && saves.length > 0 ? undefined : true);
                    }
                });
        }, 1000);

        return () => clearInterval(interval);
    }, [installStatus.installing, isInstalling, saves.length])

    const formatBytes = bytes => {
        if (!bytes || bytes < 0) {
            return '0 MB';
        }
        return `${(bytes / 1000000).toFixed(1)} MB`;
    }

    const installProgress = installStatus.total > 0
        ? Math.min(100, Math.round((installStatus.downloaded / installStatus.total) * 100))
        : 0;

    const installedVersion = installStatus.installed && !['0.0.0', '0.0.0.0'].includes(installStatus.version)
        ? installStatus.version
        : 'Not installed';
    const selectedInstallVersion = installVersionMode === 'stable'
        ? installStatus.latest_stable
        : installVersionMode === 'latest'
            ? installStatus.latest
            : customInstallVersion;
    const isChangingInstalledVersion = installStatus.installed &&
        selectedInstallVersion &&
        selectedInstallVersion !== installStatus.version;
    const needsBackupBeforeInstall = isChangingInstalledVersion && saves.length > 0 && backups.length === 0;
    const installDisabled = serverStatus.running ||
        needsBackupBeforeInstall ||
        (installVersionMode === 'custom' && !customInstallVersion);
    const updateLifecycleField = (section, field, value) => {
        setLifecycle(current => ({
            ...current,
            [section]: {
                ...current?.[section],
                [field]: value
            }
        }));
    };
    const updateLifecycleRoot = (field, value) => {
        setLifecycle(current => ({
            ...current,
            [field]: value
        }));
    };
    const interfaceAddressOptions = interfaces.flatMap(networkInterface =>
        (networkInterface.addresses || []).map(address => ({
            value: address.ip,
            label: `${networkInterface.display_name || networkInterface.name} - ${address.ip}`
        }))
    );
    const saveLifecycle = async () => {
        setIsSavingLifecycle(true);
        try {
            const saved = await server.updateLifecycle(lifecycle);
            setLifecycle(saved);
            window.flash("Server lifecycle settings saved.", "green");
        } catch (error) {
            window.flash(error?.response?.data || error.message, "red");
        } finally {
            setIsSavingLifecycle(false);
        }
    };

    return (
        <form onSubmit={handleSubmit(startServer)}>
        <Panel
            title="Server Status"
            content={
                <>
                    {serverStatus?.emulated &&
                        <div className="bg-orange text-black rounded-sm px-3 py-2 mb-2 text-sm">
                            Factorio is running under x86_64 emulation (box64) on {serverStatus.host_arch} — expect reduced performance.
                        </div>
                    }
                    <div className="lg:flex">
                    { serverStatus.running
                        ? <>
                            <div className="lg:w-1/5 mb-2">
                                <div className="font-bold">Status</div>
                                <div>{serverStatus.running ? 'Running' : 'Stopped'}</div>
                            </div>
                            <div className="lg:w-1/5 mb-2">
                                <div className="font-bold">IP</div>
                                <div>{serverStatus.bindip}</div>
                            </div>
                            <div className="lg:w-1/5 mb-2">
                                <div className="font-bold">Port</div>
                                <div>{serverStatus.port}</div>
                            </div>
                            <div className="lg:w-1/5 mb-2">
                                <div className="font-bold">Factorio Version</div>
                                <div>{installedVersion}</div>
                            </div>
                            <div className="lg:w-1/5 mb-2">
                                <div className="font-bold">Save</div>
                                <div>{serverStatus.savefile}</div>
                            </div>
                        </>
                        : <>
                            <div className="lg:w-1/5 mb-2">
                                <div className="font-bold">Status</div>
                                <div>{serverStatus.running ? 'Running' : 'Stopped'}</div>
                            </div>
                            <div className="lg:w-1/5 mb-2 mr-0 lg:mr-4">
                                <div className="font-bold">IP</div>
                                <Input
                                    defaultValue={lifecycle?.startup_profile?.bindip || "0.0.0.0"}
                                    disabled={isDisabled}
                                    list="server-bind-ip-options"
                                    register={register('ip',{required: true, pattern: /^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/})}
                                />
                                <Error error={errors.ip} message="IP is required and must be valid."/>
                            </div>
                            <div className="lg:w-1/5 mb-2 mr-0 lg:mr-4">
                                <div className="font-bold">Port</div>
                                <Input
                                    type="number"
                                    min={1}
                                    max={65535}
                                    defaultValue={lifecycle?.startup_profile?.port || "34197"}
                                    disabled={isDisabled}
                                    register={register('port',{required: true, min: 1, max: 65535})}
                                />
                                <Error error={errors.port} message="Port is required within range 1-65535"/>
                            </div>
                            <div className="lg:w-1/5 mb-2 mr-0 lg:mr-4">
                                <div className="font-bold">Factorio Version</div>
                                <div>{installedVersion}</div>
                            </div>
                            <div className="lg:w-1/5 mb-2">
                                <div className="font-bold">Save</div>
                                <div className="relative">
                                    <Select
                                        register={register('save',{required: true})}
                                        defaultValue={lifecycle?.startup_profile?.savefile || saves.find((save) => save.name.startsWith('Load Latest'))?.name}
                                        disabled={isDisabled}
                                        options={saves.map(save => new Object({
                                            value: save.name,
                                            name: save.name
                                        }))}
                                    />
                                    <Error error={errors.save} message="Save is required and must be valid."/>
                                </div>
                            </div>
                        </>
                    }
                    </div>
                </>
            }
            actions={
                <div className="md:flex">
                    {serverStatus.running
                        ? <>
                            <Button onClick={stopServer} isLoading={isStopping} isDisabled={isKilling} size="sm" className="w-full md:w-auto mb-2 md:mb-0 md:mr-2" type="default">Save & Stop Server</Button>
                            <Button onClick={killServer} isLoading={isKilling} isDisabled={isStopping} size="sm" type="danger" className="w-full md:w-auto">Kill Server</Button>
                        </>
                        : <Button isSubmit={true} isDisabled={isDisabled} isLoading={isStarting} size="sm" type="success" className="w-full md:w-auto">Start Server</Button>
                    }
                    {startError && <div className="text-red ml-0 md:ml-4 mt-2 md:mt-0">{startError}</div>}
                </div>
            }
        />
        <datalist id="server-bind-ip-options">
            {interfaceAddressOptions.map(option =>
                <option value={option.value} label={option.label} key={`${option.label}-${option.value}`}/>
            )}
        </datalist>
        <Panel
            title="Factorio Server Installation"
            className="mt-6"
            content={
                <div>
                    <div className="lg:flex">
                        <div className="lg:w-1/4 mb-2 mr-0 lg:mr-4">
                            <div className="font-bold">Installed Version</div>
                            <div>{installedVersion}</div>
                        </div>
                        <div className="lg:w-1/4 mb-2 mr-0 lg:mr-4">
                            <div className="font-bold">Base Mod Version</div>
                            <div>{installStatus.base_mod_version || 'Unknown'}</div>
                        </div>
                        <div className="lg:w-1/4 mb-2 mr-0 lg:mr-4">
                            <div className="font-bold">Latest Stable</div>
                            <div>{installStatus.latest_stable || 'Unknown'}</div>
                        </div>
                        <div className="lg:w-1/4 mb-2">
                            <div className="font-bold">Latest Experimental</div>
                            <div>{installStatus.latest || 'Unknown'}</div>
                        </div>
                    </div>
                    <div className="lg:flex mt-2">
                        <div className="lg:w-1/2 mb-2 mr-0 lg:mr-4">
                            <div className="font-bold">Save Backups</div>
                            <div>{backups.length} available</div>
                        </div>
                        <div className="lg:w-1/2 mb-2">
                            <div className="font-bold">Version Safety</div>
                            <div>
                                {needsBackupBeforeInstall
                                    ? "Create at least one save backup before changing Factorio versions."
                                    : "Ready"}
                            </div>
                        </div>
                    </div>
                    <div className="lg:flex mt-2">
                        <div className="lg:w-1/2 mb-2 mr-0 lg:mr-4">
                            <div className="font-bold">Status</div>
                            <div>{installStatus.message || 'Ready'}</div>
                        </div>
                        <div className="lg:w-1/2 mb-2">
                            <div className="font-bold">Target</div>
                            <div>{installStatus.target_version || '-'}</div>
                        </div>
                    </div>
                    {(installStatus.installing || isInstalling) &&
                        <div className="mt-4">
                            <div className="h-3 bg-gray-dark">
                                <div className="h-3 bg-green" style={{width: `${installProgress}%`}}/>
                            </div>
                            <div className="mt-2 text-sm">
                                {installStatus.total > 0
                                    ? `${installProgress}% (${formatBytes(installStatus.downloaded)} / ${formatBytes(installStatus.total)})`
                                    : installStatus.phase}
                            </div>
                        </div>
                    }
                    {installError && <div className="text-red mt-2">{installError}</div>}
                </div>
            }
            actions={
                <div className="md:flex md:items-end">
                    <div className="mb-2 md:mb-0 md:mr-2">
                        <div className="font-bold text-white">Install Version</div>
                        <div className="relative">
                            <select
                                className="shadow appearance-none border w-full py-2 px-3 text-black"
                                value={installVersionMode}
                                disabled={serverStatus.running || isInstalling || installStatus.installing}
                                onChange={event => setInstallVersionMode(event.target.value)}
                            >
                                <option value="stable">Stable</option>
                                <option value="latest">Latest experimental</option>
                                <option value="custom">Specific version</option>
                            </select>
                            <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2 text-black">
                                <svg className="fill-current h-4 w-4" xmlns="http://www.w3.org/2000/svg"
                                     viewBox="0 0 20 20">
                                    <path
                                        d="M9.293 12.95l.707.707L15.657 8l-1.414-1.414L10 10.828 5.757 6.586 4.343 8z"/>
                                </svg>
                            </div>
                        </div>
                    </div>
                    {installVersionMode === 'custom' &&
                        <div className="mb-2 md:mb-0 md:mr-2">
                            <div className="font-bold text-white">Version</div>
                            <Input
                                value={customInstallVersion}
                                disabled={serverStatus.running || isInstalling || installStatus.installing}
                                onChange={event => setCustomInstallVersion(event.target.value)}
                                placeholder="1.1.110"
                            />
                        </div>
                    }
                    <Button
                        onClick={installFactorio}
                        isLoading={isInstalling || installStatus.installing}
                        isDisabled={installDisabled}
                        size="sm"
                        type="success"
                        className="w-full md:w-auto mb-2 md:mb-0 md:mr-2"
                    >{installStatus.installed ? 'Change Version' : 'Install'}</Button>
                    {installStatus.update_available &&
                        <Button
                            onClick={updateFactorio}
                            isLoading={isInstalling || installStatus.installing}
                            isDisabled={serverStatus.running || needsBackupBeforeInstall}
                            size="sm"
                            type="default"
                            className="w-full md:w-auto"
                        >Update to {installStatus.update_version}</Button>
                    }
                </div>
            }
        />
        {lifecycle &&
            <Panel
                title="Server Lifecycle"
                className="mt-6"
                content={
                    <div>
                        <div className="lg:flex">
                            <div className="lg:w-1/4 mb-2 mr-0 lg:mr-4">
                                <div className="font-bold">Profile Save</div>
                                <Select
                                    value={lifecycle.startup_profile.savefile}
                                    onChange={event => updateLifecycleField("startup_profile", "savefile", event.target.value)}
                                    options={saves.map(save => ({value: save.name, name: save.name}))}
                                />
                            </div>
                            <div className="lg:w-1/4 mb-2 mr-0 lg:mr-4">
                                <div className="font-bold">Bind IP</div>
                                <Input value={lifecycle.startup_profile.bindip || "0.0.0.0"}
                                       list="server-bind-ip-options"
                                       onChange={event => updateLifecycleField("startup_profile", "bindip", event.target.value)}/>
                            </div>
                            <div className="lg:w-1/4 mb-2 mr-0 lg:mr-4">
                                <div className="font-bold">Port</div>
                                <Input type="number" min={1} max={65535}
                                       value={lifecycle.startup_profile.port || 34197}
                                       onChange={event => updateLifecycleField("startup_profile", "port", parseInt(event.target.value))}/>
                            </div>
                            <div className="lg:w-1/4 mb-2">
                                <div className="font-bold">Mod Pack</div>
                                <Input value={lifecycle.startup_profile.mod_pack || ""}
                                       placeholder="optional"
                                       onChange={event => updateLifecycleField("startup_profile", "mod_pack", event.target.value)}/>
                            </div>
                        </div>
                        <div className="lg:flex mt-2">
                            <div className="lg:w-1/4 mb-2 mr-0 lg:mr-4">
                                <div className="font-bold">Public</div>
                                <input type="checkbox"
                                       checked={!!lifecycle.startup_profile.public}
                                       onChange={event => updateLifecycleField("startup_profile", "public", event.target.checked)}/>
                            </div>
                            <div className="lg:w-1/4 mb-2 mr-0 lg:mr-4">
                                <div className="font-bold">LAN</div>
                                <input type="checkbox"
                                       checked={!!lifecycle.startup_profile.lan}
                                       onChange={event => updateLifecycleField("startup_profile", "lan", event.target.checked)}/>
                            </div>
                            <div className="lg:w-1/4 mb-2 mr-0 lg:mr-4">
                                <div className="font-bold">Graceful Stop Timeout</div>
                                <Input type="number" min={5}
                                       value={lifecycle.graceful_stop_timeout}
                                       onChange={event => updateLifecycleRoot("graceful_stop_timeout", parseInt(event.target.value))}/>
                            </div>
                            <div className="lg:w-1/4 mb-2">
                                <div className="font-bold">Scheduled Restart</div>
                                <input type="checkbox"
                                       checked={!!lifecycle.restart_schedule.enabled}
                                       onChange={event => updateLifecycleField("restart_schedule", "enabled", event.target.checked)}/>
                            </div>
                        </div>
                        <div className="lg:flex mt-2">
                            <div className="lg:w-1/4 mb-2 mr-0 lg:mr-4">
                                <div className="font-bold">Restart Interval Hours</div>
                                <Input type="number" min={1}
                                       value={lifecycle.restart_schedule.interval_hours}
                                       onChange={event => updateLifecycleField("restart_schedule", "interval_hours", parseInt(event.target.value))}/>
                            </div>
                            <div className="lg:w-1/4 mb-2 mr-0 lg:mr-4">
                                <div className="font-bold">Next Restart</div>
                                <div>{lifecycle.restart_schedule.next_restart || "Not scheduled"}</div>
                            </div>
                            <div className="lg:w-1/2 mb-2">
                                <div className="font-bold">Last Crash</div>
                                <div>{lifecycle.last_crash?.reason || "None"}</div>
                            </div>
                        </div>
                        <div className="mt-2">
                            <div className="font-bold">Recent Events</div>
                            {(lifecycle.events || []).slice(0, 5).map((event, index) =>
                                <div key={index} className="text-sm">
                                    {new Date(event.time).toLocaleString()} [{event.type}] {event.message}
                                </div>
                            )}
                            {(lifecycle.events || []).length === 0 && <div>None</div>}
                        </div>
                        {lifecycle.last_crash?.recent_logs?.length > 0 &&
                            <div className="mt-2">
                                <div className="font-bold">Recent Crash Logs</div>
                                <pre className="text-xs whitespace-pre-wrap">{lifecycle.last_crash.recent_logs.join("\n")}</pre>
                            </div>
                        }
                    </div>
                }
                actions={
                    <Button size="sm" type="success" isLoading={isSavingLifecycle} onClick={saveLifecycle}>
                        Save Lifecycle Settings
                    </Button>
                }
            />
        }
        </form>
    )
};

export default Controls;
