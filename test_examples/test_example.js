// Test JavaScript file for funcfinder

// Regular function
function regularFunction() {
    console.log("regular");
    return true;
}

// Async function
async function asyncFunction() {
    const data = await fetch('/api');
    return data;
}

// Generator function
function* generatorFunction() {
    yield 1;
    yield 2;
}

// Class with methods
class MyClass {
    constructor() {
        this.value = 0;
    }

    // Regular method
    regularMethod() {
        return this.value;
    }

    // Async method
    async asyncMethod() {
        const result = await somePromise();
        return result;
    }

    // Getter
    get value() {
        return this._value;
    }

    // Setter
    set value(val) {
        this._value = val;
    }

    // Generator method
    *generatorMethod() {
        yield this.value;
    }
}

// Arrow functions (won't be detected by current pattern)
const arrowFunc = () => {
    return "arrow";
};

const asyncArrowFunc = async () => {
    return await getData();
};

// Function expression (won't be detected)
const funcExpr = function() {
    return "expression";
};

// Exported function
export function exportedFunction() {
    return "exported";
}

// Export async function
export async function exportedAsyncFunction() {
    return await process();
}

// Object with methods
const obj = {
    objectMethod() {
        return "method";
    },

    async asyncObjectMethod() {
        return await load();
    }
};

// Nested function
function outerFunction() {
    function innerFunction() {
        return "inner";
    }
    return innerFunction();
}

// Function with comments
function commentedFunction() {
    // This is a comment with { braces }
    /* Block comment with "strings" */
    return "value";
}

// Function with strings
function stringFunction() {
    const str1 = "string with { braces }";
    const str2 = 'single quotes';
    const str3 = `template literal with ${variable}`;
    return str1 + str2 + str3;
}
