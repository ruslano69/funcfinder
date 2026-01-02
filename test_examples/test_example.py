#!/usr/bin/env python3
"""
Test file for Python function detection with decorators
"""

import functools
from typing import List, Optional


# Simple function without decorator
def simple_function():
    """A simple function"""
    x = 1 + 2
    return x


# Function with single decorator
@staticmethod
def static_method():
    """Static method example"""
    return "static"


# Function with multiple decorators
@lru_cache(maxsize=128)
@validate_input
def cached_function(x, y):
    """Function with multiple decorators"""
    result = x + y
    return result


# Async function with decorator
@require_auth
async def async_function(user_id: int):
    """Async function example"""
    data = await fetch_data(user_id)
    return data


# Class with decorated methods
class MyClass:
    """Example class"""

    @property
    def name(self):
        """Property decorator"""
        return self._name

    @name.setter
    def name(self, value):
        """Setter decorator"""
        self._name = value

    @classmethod
    def from_dict(cls, data: dict):
        """Class method"""
        instance = cls()
        instance.name = data.get('name')
        return instance

    @staticmethod
    def validate(input_str: str) -> bool:
        """Static method in class"""
        return len(input_str) > 0


# Nested functions
def outer_function(x):
    """Outer function with nested function"""

    def inner_function(y):
        """Inner nested function"""
        return x + y

    result = inner_function(10)
    return result


# Function with complex decorator
@app.route('/api/users/<int:user_id>')
@require_permission('read:users')
def get_user(user_id):
    """REST API endpoint"""
    user = database.get_user(user_id)
    return jsonify(user)


# Generator function with decorator
@log_execution_time
def fibonacci_generator(n):
    """Generate Fibonacci numbers"""
    a, b = 0, 1
    for _ in range(n):
        yield a
        a, b = b, a + b


# Async generator
async def async_generator():
    """Async generator example"""
    for i in range(10):
        await asyncio.sleep(0.1)
        yield i


# Function with docstring and comments
def complex_function(
    param1: str,
    param2: Optional[int] = None,
    param3: List[str] = None
) -> dict:
    """
    Complex function with multiline signature

    Args:
        param1: First parameter
        param2: Second parameter
        param3: Third parameter

    Returns:
        Dictionary with results
    """
    # Initialize result
    result = {}

    # Process param1
    if param1:
        result['param1'] = param1.upper()

    # Process param2
    if param2:
        result['param2'] = param2 * 2

    # Process param3
    if param3:
        result['param3'] = ', '.join(param3)

    return result


# Empty function
def empty_function():
    pass


# One-liner function
def one_liner(): return 42


if __name__ == '__main__':
    print("Testing Python functions")
    print(simple_function())
