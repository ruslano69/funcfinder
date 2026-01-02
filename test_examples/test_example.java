package com.example;

import java.util.List;
import java.util.ArrayList;
import java.util.Arrays;

/**
 * Test class for Java function detection
 */
public class TestExample {

    private int value;

    // Constructor
    public TestExample() {
        this.value = 0;
    }

    // Simple method
    public void add(int n) {
        value += n;
    }

    // Method with return value
    public int getValue() {
        return value;
    }

    // Static method
    public static int multiply(int a, int b) {
        return a * b;
    }

    // Method with multiple parameters
    public String formatMessage(String template, Object... args) {
        return String.format(template, args);
    }

    // Generic method
    public <T> T getFirst(List<T> list) {
        return list.isEmpty() ? null : list.get(0);
    }

    // Method with throws clause
    public void processFile(String filename) throws IOException {
        // Process file
        throw new IOException("File not found");
    }

    // Private method
    private void helperMethod() {
        System.out.println("Helper");
    }

    // Protected method
    protected void setup() {
        value = 0;
    }

    // Main method
    public static void main(String[] args) {
        TestExample example = new TestExample();
        example.add(10);
        System.out.println("Value: " + example.getValue());

        System.out.println("Product: " + multiply(5, 3));
    }

    // Inner class
    private static class Helper {
        public void assist() {
            System.out.println("Assisting");
        }
    }
}
