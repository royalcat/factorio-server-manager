import React from "react";

const Input = ({
                   register,
                   placeholder = undefined,
                   type = "text",
                   defaultValue = undefined,
                   hasAutoComplete = true,
                   onKeyDown = () => undefined,
                   onChange = undefined,
                   onBlur = undefined,
                   onFocus = undefined,
                   list = undefined,
                   min = undefined,
                   max = undefined,
                   step = undefined,
                   value = undefined,
                   disabled = false
               }) => {
    return (
        <input
            className="shadow appearance-none border w-full py-2 px-3 text-black"
            placeholder={placeholder}
            {...register}
            type={type}
            onKeyDown={onKeyDown}
            {...(onChange ? {onChange} : {})}
            {...(onBlur ? {onBlur} : {})}
            {...(onFocus ? {onFocus} : {})}
            autoComplete={hasAutoComplete ? "on" : "off"}
            list={list}
            defaultValue={defaultValue}
            min={min}
            max={max}
            step={step}
            value={value}
            disabled={disabled}
        />
    )
}

export default Input;
