// Test TypeScript file for funcfinder

interface User {
    name: string;
    age: number;
}

// Function with type annotations
function greet(name: string): string {
    return `Hello, ${name}`;
}

// Async function with types
async function fetchUser(id: number): Promise<User> {
    const response = await fetch(`/api/users/${id}`);
    return response.json();
}

// Generic function
function identity<T>(arg: T): T {
    return arg;
}

// Class with typed methods
class UserService {
    private users: User[] = [];

    constructor(users: User[]) {
        this.users = users;
    }

    getUser(id: number): User | undefined {
        return this.users.find(u => u.id === id);
    }

    async saveUser(user: User): Promise<void> {
        await this.db.save(user);
    }

    updateUser(id: number, data: Partial<User>): void {
        const user = this.getUser(id);
        if (user) {
            Object.assign(user, data);
        }
    }
}

// Arrow function with types
const add = (a: number, b: number): number => a + b;

// Export function
export function calculateTotal(items: number[]): number {
    return items.reduce((sum, item) => sum + item, 0);
}

// Type guard function
function isUser(obj: any): obj is User {
    return obj && typeof obj.name === 'string';
}
