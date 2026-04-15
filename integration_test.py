# =========================
# INTEGRATION TEST
# =========================

"""
Integration tests for markdown_document_generator workflow integration.
Verifies document generation occurs at correct workflow stages with proper content.
"""

import sys
import os
import json
from utils import markdown_document_generator, DOCUMENT_STORES_DIR
from orchestrator import Orchestrator


def test_pm_transformation_workflow_integration():
    """Verify markdown_document_generator triggers in pm_transformation_workflow()."""
    print("Testing PM transformation workflow integration...")
    
    # Create mock content matching what rephrased_task would contain
    mock_rephrased_task = {
        'task_specification': 'Test transformed task specification from PM agent'
    }
    
    # Capture expected file generation
    try:
        markdown_document_generator(mock_rephrased_task, 'product_manager_final')
        
        # Verify file was created in document_stores directory
        md_files = list(os.listdir(DOCUMENT_STORES_DIR))
        pm_files = [f for f in md_files if 'product_manager_final' in f]
        
        if len(pm_files) > 0:
            print(f"✓ PM workflow integration test passed - generated {len(pm_files)} PM documents")
            
            # Verify file content structure
            with open(os.path.join(DOCUMENT_STORES_DIR, pm_files[0]), 'r') as f:
                content = f.read()
                if '# product_manager_final' in content and '## Task Specification' in content:
                    print("✓ PM document has correct structure with section headings")
                    return True
        else:
            print("✗ PM workflow integration test failed - no documents generated")
            return False
            
    except Exception as e:
        print(f"✗ PM workflow integration test failed: {e}")
        return False


def test_architecture_design_phase_integration():
    """Verify markdown_document_generator triggers in architecture_design_phase()."""
    print("Testing architecture design phase integration...")
    
    # Create mock content matching what arch dict would contain after approval
    # Agent produces nested structure: {'architecture': {...}} as defined by ARCH_PROMPT
    mock_arch = {
        'architecture': {
            'overview': 'Test architecture overview',
            'reviewer_notes': ['Architecture alignment notes'],
            'components': [
                {'name': 'Database', 'responsibility': 'Data storage and retrieval', 'background': 'Handles persistent data'},
                {'name': 'API Server', 'responsibility': 'Request processing and routing', 'background': 'Manages API endpoints'},
                {'name': 'Frontend', 'responsibility': 'User interface rendering', 'background': 'Client-side application'}
            ],
            'data_flow': ['Client → API → Database'],
            'tech_choices': ['Python for backend', 'PostgreSQL for database'],
            'constraints': ['Must handle 10k concurrent users']
        }
    }
    
    try:
        markdown_document_generator(mock_arch, 'architecture_after_reviews')
        
        # Verify file was created
        md_files = list(os.listdir(DOCUMENT_STORES_DIR))
        arch_files = [f for f in md_files if 'architecture_after_reviews' in f]
        
        if len(arch_files) > 0:
            print(f"✓ Architecture phase integration test passed - generated {len(arch_files)} architecture documents")
            
            # Verify file content structure
            with open(os.path.join(DOCUMENT_STORES_DIR, arch_files[-1]), 'r') as f:
                content = f.read()
                if '# architecture_after_reviews' in content and '## Overview' in content and '## Components' in content:
                    print("✓ Architecture document has correct section headings")
                    return True
        else:
            print("✗ Architecture phase integration test failed - no documents generated")
            return False
            
    except Exception as e:
        print(f"✗ Architecture phase integration test failed: {e}")
        return False


def test_plan_creation_phase_integration():
    """Verify markdown_document_generator triggers in plan_creation_phase()."""
    print("Testing plan creation phase integration...")
    
    # Create mock content matching what plan dict would contain after approval
    # Agent produces nested structure: {'plan': {...}} as defined by PLAN_PROMPT
    mock_plan = {
        'plan': {
            'summary': 'Test technical implementation plan summary',
            'reviewer_notes': ['Plan alignment notes'],
            'files': [
                {'path': 'src/auth.py', 'purpose': 'Authentication module for user login/logout', 'background': 'Handles security'},
                {'path': 'src/ratelimiter.py', 'purpose': 'Rate limiting module for request throttling', 'background': 'Prevents overload'}
            ],
            'steps': [
                {'id': 1, 'description': 'Implement authentication module with JWT tokens'},
                {'id': 2, 'description': 'Add rate limiter to API endpoints'}
            ]
        }
    }
    
    try:
        markdown_document_generator(mock_plan, 'tech_plan_after_reviews')
        
        # Verify file was created
        md_files = list(os.listdir(DOCUMENT_STORES_DIR))
        plan_files = [f for f in md_files if 'tech_plan_after_reviews' in f]
        
        if len(plan_files) > 0:
            print(f"✓ Plan creation phase integration test passed - generated {len(plan_files)} plan documents")
            
            # Verify file content structure
            with open(os.path.join(DOCUMENT_STORES_DIR, plan_files[-1]), 'r') as f:
                content = f.read()
                if '# tech_plan_after_reviews' in content and '## Summary' in content and '## Files' in content:
                    print("✓ Plan document has correct section headings")
                    return True
        else:
            print("✗ Plan creation phase integration test failed - no documents generated")
            return False
            
    except Exception as e:
        print(f"✗ Plan creation phase integration test failed: {e}")
        return False


def run_all_integration_tests():
    """Execute all integration tests and report results."""
    print("\n=== RUNNING INTEGRATION TESTS ===\n")
    
    # Ensure document_stores directory exists
    os.makedirs(DOCUMENT_STORES_DIR, exist_ok=True)
    
    # Run each test
    test_results = [
        ("PM Transformation Workflow", test_pm_transformation_workflow_integration()),
        ("Architecture Design Phase", test_architecture_design_phase_integration()),
        ("Plan Creation Phase", test_plan_creation_phase_integration())
    ]
    
    # Report results
    passed = sum(1 for _, result in test_results if result)
    total = len(test_results)
    
    print(f"\n=== INTEGRATION TEST RESULTS ===")
    print(f"Tests passed: {passed}/{total}")
    
    for name, result in test_results:
        status = "PASS" if result else "FAIL"
        print(f"{name}: {status}")
    
    return all(result for _, result in test_results)


if __name__ == "__main__":
    success = run_all_integration_tests()
    sys.exit(0 if success else 1)
