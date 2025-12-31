// Test file for generator functions and arrow functions

// Generator functions
function* simpleGenerator() {
    yield 1;
    yield 2;
    yield 3;
}

function* fibonacciGenerator() {
    let a = 0, b = 1;
    while (true) {
        yield a;
        [a, b] = [b, a + b];
    }
}

// Async generator
async function* asyncGenerator() {
    yield await Promise.resolve(1);
    yield await Promise.resolve(2);
}

// Export generator
export function* exportedGenerator() {
    yield "exported";
}

// Arrow functions with body
const arrowFunc = () => {
    return "arrow";
};

const arrowWithParams = (x, y) => {
    return x + y;
};

const asyncArrow = async () => {
    const data = await fetch('/api');
    return data;
};

const asyncArrowWithParams = async (id) => {
    const user = await fetchUser(id);
    return user;
};

// Arrow function assigned to const
const processData = (items) => {
    const result = items.map(x => x * 2);
    return result;
};

// Let and var variants
let letArrow = () => {
    console.log("let arrow");
};

var varArrow = () => {
    console.log("var arrow");
};

// Class with generator method
class MyClass {
    *classGenerator() {
        yield 1;
        yield 2;
    }

    async *asyncClassGenerator() {
        yield await Promise.resolve(1);
    }
}

// Object with methods
const obj = {
    *objectGenerator() {
        yield "object";
    },

    arrow: () => {
        return "object arrow";
    }
};
