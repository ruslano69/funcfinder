module ffd_config_table;

// The embedded schema has only strings, booleans, string arrays, objects and
// nulls. All references point into immutable generated data, never a JSON DOM.
enum ConfigKind { null_, text, flag, strings, object_ }

struct ConfigEntry {
    string key;
    ConfigKind kind;
    string text;
    bool flag;
    immutable(string)[] strings;
    immutable(ConfigEntry)[] children;

    ConfigEntry opIndex(string name) const @nogc nothrow {
        // genconfig emits object keys in lexical order. Lookup needs neither a
        // hash table nor heap allocation.
        size_t low, high = children.length;
        while (low < high) {
            auto middle = low + (high-low)/2;
            if (children[middle].key < name) low = middle+1;
            else high = middle;
        }
        if (low < children.length && children[low].key == name) return children[low];
        return ConfigEntry.init;
    }

    string str() const @nogc nothrow { return text; }
}
