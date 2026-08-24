package domain

import "strings"

func (r AcceptanceRuleSet) Validate() error {
	_, err := r.NormalizeAndValidate()
	return err
}

func (r AcceptanceRuleSet) NormalizeAndValidate() (AcceptanceRuleSet, error) {
	out := r
	out.ID = strings.TrimSpace(out.ID)
	problems := &ValidationErrors{}
	if out.ID == "" {
		problems.Add("acceptanceRuleSet.id", "验收规则集标识不能为空")
	}
	if out.Version < 1 {
		problems.Add("acceptanceRuleSet.version", "规则版本必须大于零")
	}
	if len(out.Rules) == 0 {
		problems.Add("acceptanceRuleSet.rules", "至少需要一条验收规则")
	}
	seen := make(map[string]bool)
	for i := range out.Rules {
		rule := &out.Rules[i]
		rule.ID, rule.Name = strings.TrimSpace(rule.ID), strings.TrimSpace(rule.Name)
		field := "acceptanceRuleSet.rules[" + decimalIndex(i) + "]"
		if rule.ID == "" {
			problems.Add(field+".id", "规则标识不能为空")
		}
		if rule.Name == "" {
			problems.Add(field+".name", "规则名称不能为空")
		}
		if seen[rule.ID] && rule.ID != "" {
			problems.Add(field+".id", "规则标识不能重复")
		}
		seen[rule.ID] = true
		if rule.MinVoltageKV <= 0 || rule.MaxVoltageKV <= 0 || rule.MinVoltageKV > rule.MaxVoltageKV {
			problems.Add(field+".voltage", "管电压上下限必须为正数且下限不得高于上限")
		}
		if rule.MaxDefectSizeMM <= 0 {
			problems.Add(field+".maxDefectSizeMM", "最大缺陷尺寸必须为正数")
		}
		views := make(map[string]bool)
		for j, value := range rule.RequiredViews {
			value = strings.TrimSpace(value)
			rule.RequiredViews[j] = value
			if value == "" {
				problems.Add(field+".requiredViews["+decimalIndex(j)+"]", "必需视图不能为空")
			} else if views[value] {
				problems.Add(field+".requiredViews["+decimalIndex(j)+"]", "同一规则内必需视图不能重复")
			}
			views[value] = true
		}
		zones := make(map[string]bool)
		for j, value := range rule.RequiredZones {
			value = strings.TrimSpace(value)
			rule.RequiredZones[j] = value
			if value == "" {
				problems.Add(field+".requiredZones["+decimalIndex(j)+"]", "必需区域不能为空")
			} else if zones[value] {
				problems.Add(field+".requiredZones["+decimalIndex(j)+"]", "同一规则内必需区域不能重复")
			}
			zones[value] = true
		}
	}
	if !problems.Empty() {
		return out, problems
	}
	return out, nil
}

func decimalIndex(value int) string {
	if value == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}

func (r AcceptanceRuleSet) HasRule(id string) bool {
	for _, rule := range r.Rules {
		if rule.ID == id {
			return true
		}
	}
	return false
}
