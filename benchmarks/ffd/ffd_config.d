module ffd_config;

public import ffd_config_table : ConfigEntry, ConfigKind;
import ffd_config_data : configRoot;
import std.json : JSONValue;

ConfigEntry rootConfig() @nogc nothrow { return configRoot; }
ConfigEntry languageConfig(string key) @nogc nothrow { return configRoot["languages"][key]; }

// Existing native parser APIs retain JSONValue settings. Build only the selected
// language's adapter before workers start; no embedded JSON text is parsed.
JSONValue backendConfig(string key) {
    return configJSON(languageConfig(key));
}

private JSONValue configJSON(ConfigEntry entry) {
    final switch (entry.kind) {
        case ConfigKind.null_: return JSONValue(null);
        case ConfigKind.text: return JSONValue(entry.text);
        case ConfigKind.flag: return JSONValue(entry.flag);
        case ConfigKind.strings:
            JSONValue[] values;
            foreach (v; entry.strings) values ~= JSONValue(v);
            return JSONValue(values.length ? values : new JSONValue[0]);
        case ConfigKind.object_:
            JSONValue[string] values;
            foreach (v; entry.children) values[v.key] = configJSON(v);
            return JSONValue(values);
    }
}

unittest {
    import std.json : parseJSON;
    auto expected = parseJSON(import("ffd_config.json"));
    // Full schema equality includes every unused field, null vs empty arrays,
    // booleans, Unicode and escaped regular-expression strings.
    assert(configJSON(rootConfig()) == expected);
    // Settings mirror the Go registry; the native D registry remains 15 languages.
    assert(rootConfig()["languages"].children.length == 16);
    assert(languageConfig("goose")["extensions"].strings == [".goose"]);
    assert(languageConfig("go")["extensions"].strings == [".go"]);
    assert(languageConfig("missing").kind == ConfigKind.null_);
}
