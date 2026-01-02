using System;
using System.Collections.Generic;
using System.Linq;

namespace TestNamespace
{
    // Simple class with methods
    public class Calculator
    {
        private int value;

        public Calculator()
        {
            value = 0;
        }

        public void Add(int n)
        {
            value += n;
        }

        public int GetValue()
        {
            return value;
        }

        // Static method
        public static int Multiply(int a, int b)
        {
            return a * b;
        }

        // Property
        public int Value
        {
            get { return value; }
            set { this.value = value; }
        }

        // Async method
        public async Task<string> FetchDataAsync(int id)
        {
            await Task.Delay(100);
            return $"Data for ID: {id}";
        }

        // Generic method
        public T GetFirst<T>(List<T> list)
        {
            return list.FirstOrDefault();
        }
    }

    // Extension method
    public static class StringExtensions
    {
        public static string Reverse(this string str)
        {
            char[] arr = str.ToCharArray();
            Array.Reverse(arr);
            return new string(arr);
        }
    }

    // Program class
    class Program
    {
        static void Main(string[] args)
        {
            Calculator calc = new Calculator();
            calc.Add(10);
            Console.WriteLine($"Value: {calc.GetValue()}");

            string text = "Hello";
            Console.WriteLine(text.Reverse());
        }

        // Helper method
        static void PrintNumbers(params int[] numbers)
        {
            foreach (var num in numbers)
            {
                Console.Write($"{num} ");
            }
            Console.WriteLine();
        }
    }
}
