#!/usr/bin/env python3
"""
Module docstring with 'quotes' and "double quotes"
Should not be counted as code
"""

def example_function():
    """Function docstring
    Multiple lines
    Should not count as code
    """
    print("Hello World")  # This IS code

def another_func():
    '''Another docstring style'''
    return True  # This IS code

# Result:
# Code lines: ~4 (not 15!)
# - def example_function():
# - print("Hello World")
# - def another_func():
# - return True
