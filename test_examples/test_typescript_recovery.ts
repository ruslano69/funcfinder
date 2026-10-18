interface Port {
    catch(value: number): number;
}
class Box {
    static create<
        T extends Array<Array<number>>
    >(
        value: T
    ): T { return value; }
    returns<T extends Array<Array<number>>>(value: T): T { return value; }
}
const port = {
    async "quoted"() { return 1; },
    async run(
        value: number
    ) { return value; },
    catch() { return this; },
};
export /* marker */ function $constructor<T>(
    value: T
): T { return value; }
const record =
    (level: string) =>
    (tag: unknown): number =>
        entries.push({ level, tag });
const disk: typeof import("./api").reader =
    async () => ({
        value: 1,
    });
function control(value: string) {
    switch (value) {
        case "a": {
            if (value) { call(); }
            for (const item of items) { call(item); }
        }
    }
}
