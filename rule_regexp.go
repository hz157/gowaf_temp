package gowaf

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
)

type RuleItem struct {
	JsonGroupRule
	reg *regexp.Regexp
}

type Rule struct {
	Type     string
	Status   string
	RuleName string
	Desc     string
	Rule     []*RuleItem
}

type RuleList struct {
	Rules []*Rule
	mutex sync.RWMutex
}

func NewRuleList() *RuleList {
	return &RuleList{}
}

func (r *RuleList) HandleRule(j *JSONRule) {
	if j.Status == "invalid" {
		r.Remove(j.RuleName)
		return
	} else if j.Status == "valid" {
		rule := &Rule{
			Type:     j.Type,
			Status:   j.Status,
			RuleName: j.RuleName,
			Desc:     j.Desc,
		}

		for _, item := range j.Rule {
			ruleItem := &RuleItem{
				JsonGroupRule: item,
			}
			var err error
			ruleItem.reg, err = regexp.Compile(strings.ToUpper(item.Val))
			if err != nil {
				// 处理错误，例如打印错误信息
				errMsg := fmt.Sprintf("%s, Error compiling regex '%s': %v", j.RuleName, item.Val, err)
				InsertEgLog("ERROR", "Server-Regexp", errMsg)
				return
			} else {
				rule.Rule = append(rule.Rule, ruleItem)
			}

		}

		log.Println("RuleList add rule :", rule)
		r.Add(rule)
	}
}

func (r *RuleList) CleanRules() {
	r.mutex.Lock()
	r.Rules = r.Rules[:0:0]
	r.mutex.Unlock()
}

func (r *RuleList) CheckRequest(req *WafHttpRequest) *WafProxyResp {
	r.mutex.RLock()

	for _, item := range r.Rules {
		if shoot, resp := item.CheckRequest(req); shoot {
			r.mutex.RUnlock()
			return resp
		}
	}

	r.mutex.RUnlock()
	return SuccessResp
}

// 查询name的规则是否存在
func (r *RuleList) Exist(name string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, item := range r.Rules {
		if item.RuleName == name {
			return true
		}
	}

	return false
}

func (r *RuleList) Add(rule *Rule) {
	r.mutex.Lock()
	r.Rules = append(r.Rules, rule)
	r.mutex.Unlock()

	return
}

func (r *RuleList) Remove(name string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for index, item := range r.Rules {
		if item.RuleName == name {
			r.Rules = append(r.Rules[:index], r.Rules[index+1:]...)
			return
		}
	}
	return
}

func (r *Rule) CheckRequest(req *WafHttpRequest) (bool, *WafProxyResp) {
	//必须所有RuleItem都满足，才算命中这一条规则
	for _, item := range r.Rule {
		if !item.CheckRequest(req) {
			return false, SuccessResp
		}
	}

	log.Println(*req, " shoot ", *r)

	return true, &WafProxyResp{
		RetCode:  WAF_INTERCEPT,
		RuleName: r.RuleName,
		Desc:     r.Desc,
	}
}

func GetFieldFromReq(req *WafHttpRequest, field string) string {
	switch field {
	case "Host":
		return strings.ToUpper(req.Host)
	case "Referer", "User-Agent", "Content-Type":
		if len(req.Header[field]) > 0 {
			return strings.ToUpper(req.Header[field][0])
		}
	case "Url":
		return strings.ToUpper(req.Url)
	case "Body":
		return strings.ToUpper(string(req.Body))
	}
	return ""
}

// 对正则进行一次预编译
func (r *RuleItem) CompileReg() (err error) {
	r.reg, err = regexp.Compile(r.Val)
	return
}

func (r *RuleItem) CheckRequest(req *WafHttpRequest) bool {
	Val := GetFieldFromReq(req, r.Field)
	if r.Empty {
		return Val == ""
	}

	shoot := len(r.reg.FindString(Val)) > 0

	if r.Op == "is" {
		return shoot
	} else {
		return !shoot
	}
}
