// Test C# verbatim strings
using System;

class Program {
    void Example() {
        // Regular string
        string regular = "C:\\Users\\Test";

        // Verbatim string (should be removed correctly)
        string verbatim = @"C:\Users\Test";

        // Verbatim with escaped quotes
        string quoted = @"He said ""Hello""";

        // Verbatim multiline
        string multiline = @"Line 1
Line 2
Line 3";

        // This should be detected as code
        Console.WriteLine(regular);
        Console.WriteLine(verbatim);
        Console.WriteLine(quoted);
    }
}
